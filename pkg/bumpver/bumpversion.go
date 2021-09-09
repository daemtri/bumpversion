package bumpver

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
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
	cryptossh "golang.org/x/crypto/ssh"
)

func Execute(logger *log.Logger, config *Config) error {
	publicKeys, err := ssh.NewPublicKeysFromFile(config.GitSSHKeyUser, config.GitSSHKey, config.GitSSHKeyPassword)
	if err != nil {
		return fmt.Errorf("NewPublicKeys error: %w", err)
	}
	if config.GitSSHSkipVerifyHostKey {
		publicKeys.HostKeyCallbackHelper.HostKeyCallback = func(hostname string, remote net.Addr, key cryptossh.PublicKey) error {
			return nil
		}
	}

	repo, err := git.PlainClone(config.GitCloneDir, false, &git.CloneOptions{
		URL:      config.GitURL,
		Progress: os.Stdout,
		Auth:     publicKeys,
	})

	if err != nil {
		return fmt.Errorf("git.PlainClone error: %w", err)
	}
	t, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("open worktree error: %w", err)
	}
	if err := BumpRepoImageVersion(logger, t.Filesystem, config.Path, config.Image, config.Tag); err != nil {
		return fmt.Errorf("bumpRepoImageVersion error: %w", err)
	}
	status, err := t.Status()
	if err != nil {
		return fmt.Errorf("git status error: %w", err)
	}
	if status.IsClean() {
		return errors.New("git status: no changes")
	}
	if err := t.AddGlob("."); err != nil {
		return fmt.Errorf("git add . error: %w", err)
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
		return fmt.Errorf("git commit error: %w", err)
	}
	logger.Println("commit hash", h)
	if err := repo.Push(&git.PushOptions{
		Auth: publicKeys,
	}); err != nil {
		return fmt.Errorf("git push error: %w", err)
	}

	logger.Println("git push: success")

	return nil
}

func BumpRepoImageVersion(logger *log.Logger, repo billy.Filesystem, path, image, tag string) error {
	infos, err := repo.ReadDir(path)
	if err != nil {
		return err
	}
	for i := range infos {
		fname := repo.Join(path, infos[i].Name())
		if infos[i].IsDir() {
			if err := BumpRepoImageVersion(logger, repo, fname, image, tag); err != nil {
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
		if n, err := BumpYamlImageVersion(yamlFile, image, tag); err != nil {
			if errors.Is(err, yaml.ErrNotFoundNode) {
				continue
			}
			logger.Println("bumpversion:", fname, "error:", err)
		} else {
			if n > 0 {
				logger.Println("bumpversion:", fname, "success", n)
			}
		}
	}

	return nil
}

func BumpYamlImageVersion(yamlFile billy.File, image, tag string) (uint, error) {
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
