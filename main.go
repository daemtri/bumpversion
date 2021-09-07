package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	cli "github.com/jawher/mow.cli"
)

var config = struct {
	GitURL            string
	GitSSHKey         string
	GitSSHKeyUser     string
	GitSSHKeyPassword string
	GitCommitAuthor   string
	GitCommitEmail    string
	GitCloneDir       string

	Image string
	Tag   string
}{}

func init() {
	log.Println("try load ssh_private_key file to env BV_GIT_PRIVATE_KEY")
	f, err := os.Open("ssh_private_key")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("skip : ssh_private_key file not exists")
		} else {
			log.Println("load ssh_private_key error", err)
		}
		return
	}
	defer f.Close()
	privateKey, err := ioutil.ReadAll(f)
	if err != nil {
		log.Println("read ssh_private_key error", err)
		return
	}
	os.Setenv("BV_GIT_PRIVATE_KEY", string(privateKey))
	log.Println("load ssh_private_key file to env BV_GIT_PRIVATE_KEY success")
}

func main() {
	// create an app
	app := cli.App("bumpversion", "bump the image version in the k8s resource file in the GIT repository")
	app.Version("v version", "1.0.0")
	app.Spec = "[--git-url] [--git-clone-dir] [--git-ssh-key] [--git-ssh-key-user] [--git-ssh-key-password] [-i=<image> [-t=<tag>]]"
	app.StringPtr(&config.GitURL, cli.StringOpt{
		Name:   "git-url",
		Desc:   "k8s resource git url(currently, only SSH protocol is supported)",
		EnvVar: "BV_GIT_URL",
	})
	app.StringPtr(&config.GitCloneDir, cli.StringOpt{
		Name:   "git-clone-dir",
		Desc:   "k8s resource git clone dir",
		EnvVar: "BV_GIT_CLONE_DIR",
		Value:  "./resource-repo",
	})
	app.StringPtr(&config.GitSSHKey, cli.StringOpt{
		Name:   "git-ssh-key",
		Desc:   "git repo SSH private key",
		EnvVar: "BV_GIT_PRIVATE_KEY",
	})
	app.StringPtr(&config.GitSSHKeyUser, cli.StringOpt{
		Name:   "git-ssh-key-user",
		Desc:   "git repo SSH private key user",
		EnvVar: "BV_GIT_PRIVATE_KEY_USER",
		Value:  "git",
	})
	app.StringPtr(&config.GitSSHKeyPassword, cli.StringOpt{
		Name:   "git-ssh-key-password",
		Desc:   "git repo SSH private key password",
		EnvVar: "BV_GIT_PRIVATE_KEY_PASSWORD",
	})
	app.StringPtr(&config.Image, cli.StringOpt{
		Name:   "i image",
		Desc:   "image that need to be updated",
		EnvVar: "BV_IMAGE",
	})
	app.StringPtr(&config.Tag, cli.StringOpt{
		Name:   "t tag",
		Desc:   "image version that needs to be updated",
		EnvVar: "BV_TAG",
	})

	app.Action = execute
	if err := app.Run(os.Args); err != nil {
		log.Fatalln(err)
	}
}

func execute() {
	publicKeys, err := ssh.NewPublicKeys(config.GitSSHKeyUser, []byte(config.GitSSHKey), config.GitSSHKeyPassword)
	if err != nil {
		log.Println("BV_GIT_PRIVATE_KEY", base64.RawStdEncoding.EncodeToString([]byte(config.GitSSHKey)))
		log.Fatalln("NewPublicKeys", err)
	}
	repo, err := git.PlainClone(config.GitCloneDir, false, &git.CloneOptions{
		URL:      config.GitURL,
		Progress: os.Stdout,
		Auth:     publicKeys,
	})
	if err != nil {
		panic(err)
	}
	t, err := repo.Worktree()
	if err != nil {
		panic(err)
	}
	if err := bumpRepoImageVersion(t.Filesystem, ".", config.Image, config.Tag); err != nil {
		log.Fatalln(err)
	}
	status, err := t.Status()
	if err != nil {
		log.Fatalln(err)
	}
	if status.IsClean() {
		log.Fatalln("no changes")
	}
	if err := t.AddGlob("."); err != nil {
		log.Fatalln(err)
	}
	h, err := t.Commit(fmt.Sprintf("bumpversion(%s):%s", config.Image, config.Tag), &git.CommitOptions{
		All: true,
		Author: &object.Signature{
			Name:  config.GitCommitAuthor,
			Email: config.GitCommitEmail,
			When:  time.Now(),
		},
	})
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("commit hash", h)
	if err := repo.Push(&git.PushOptions{
		Auth: publicKeys,
	}); err != nil {
		log.Fatalln(err)
	}
	log.Println("git push successful")
}

func bumpRepoImageVersion(repo billy.Filesystem, path, image, tag string) error {
	infos, err := repo.ReadDir(path)
	if err != nil {
		return err
	}
	for i := range infos {
		fname := repo.Join(path, infos[i].Name())
		if infos[i].IsDir() {
			if err := bumpRepoImageVersion(repo, fname, image, tag); err != nil {
				return err
			}
			continue
		}

		if !strings.HasSuffix(fname, ".yaml") {
			continue
		}
		yamlFile, err := repo.OpenFile(fname, os.O_RDWR, os.ModePerm)
		if err != nil {
			return fmt.Errorf("open file error %s:%s", fname, err.Error())
		}
		if n, err := bumpYamlImageVersion(yamlFile, image, tag); err != nil {
			if errors.Is(err, yaml.ErrNotFoundNode) {
				continue
			}
			log.Println("bumpversion:", fname, "error:", err)
		} else {
			if n > 0 {
				log.Println("bumpversion:", fname, "success", n)
			}
		}
	}

	return nil
}

func bumpYamlImageVersion(yamlFile billy.File, image, tag string) (uint, error) {
	bs, err := ioutil.ReadAll(yamlFile)
	defer yamlFile.Close()
	if err != nil {
		return 0, err
	}

	astFile, err := parser.ParseBytes(bs, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	kind, err := getResourceKind(astFile)
	if err != nil {
		return 0, err
	}

	var imagesPath *yaml.Path
	switch kind {
	case "Deployment", "StatefulSet", "Job":
		imagesPath, _ = yaml.PathString("$.spec.template.spec.containers[*].image")
	case "CronJob":
		imagesPath, _ = yaml.PathString("$.spec.jobTemplate.spec.template.spec.containers[*].image")
	default:
		return 0, nil
	}

	node, err := imagesPath.FilterFile(astFile)
	if err != nil {
		return 0, err
	}

	var updated uint
	imageListNodes := node.(*ast.SequenceNode).Values
	for i := range imageListNodes {
		t := imageListNodes[i].(*ast.StringNode)
		imageAndOldTag := strings.SplitN(t.Value, ":", 2)
		if imageAndOldTag[0] != image || imageAndOldTag[1] == tag {
			continue
		}

		t.Value = image + ":" + tag
		updated++
	}
	if updated <= 0 {
		return 0, nil
	}

	if _, err := yamlFile.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	if err := yamlFile.Truncate(int64(len(bs))); err != nil {
		return 0, err
	}

	if _, err := io.Copy(yamlFile, astFile); err != nil {
		return 0, err
	}
	return updated, nil
}

func getResourceKind(astFile *ast.File) (string, error) {
	kindPath, err := yaml.PathString("$.kind")
	if err != nil {
		return "", err
	}
	kindNode, err := kindPath.FilterFile(astFile)
	if err != nil {
		return "", err
	}
	return kindNode.GetToken().Value, nil
}
