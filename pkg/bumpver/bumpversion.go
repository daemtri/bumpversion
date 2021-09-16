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

var (
	ErrorImageNotFound = errors.New("image not found")
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

		if err := BumpFileImageVersion(logger, repo, fname, image, tag); err != nil {
			if !errors.Is(err, ErrorImageNotFound) {
				logger.Println("bumpversion:", fname, "error:", err)
			}
			continue
		}
		logger.Println("bumpversion:", fname, "SUCCEEDED")
	}

	return nil
}

func BumpFileImageVersion(logger *log.Logger, repo billy.Filesystem, path, image, tag string) error {
	yamlFile, err := repo.OpenFile(path, os.O_RDWR, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open file error %s:%s", path, err.Error())
	}
	defer yamlFile.Close()

	bs, err := ioutil.ReadAll(yamlFile)
	if err != nil {
		return err
	}

	result, err := BumpYamlImageVersion(bs, image, tag)
	if err != nil {
		return err
	}

	if _, err := yamlFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if yamlFile.Truncate(0); err != nil {
		return err
	}
	if _, err := yamlFile.Write([]byte(result)); err != nil {
		return err
	}

	return nil
}

func BumpYamlImageVersion(yamlBytes []byte, image, tag string) (string, error) {
	astFile, err := parser.ParseBytes(yamlBytes, parser.ParseComments)
	if err != nil {
		return "", err
	}
	kind, err := getResourceKind(astFile)
	if err != nil {
		if errors.Is(err, yaml.ErrNotFoundNode) {
			return "", ErrorImageNotFound
		}
		return "", err
	}

	var imagesPath *yaml.Path
	switch kind {
	case "Deployment", "StatefulSet", "Job":
		imagesPath, _ = yaml.PathString("$.spec.template.spec.containers[*].image")
	case "CronJob":
		imagesPath, _ = yaml.PathString("$.spec.jobTemplate.spec.template.spec.containers[*].image")
	default:
		return "", ErrorImageNotFound
	}

	node, err := imagesPath.FilterFile(astFile)
	if err != nil {
		if errors.Is(err, yaml.ErrNotFoundNode) {
			return "", ErrorImageNotFound
		}
		return "", err
	}

	var updated uint
	containers, ok := node.(*ast.SequenceNode)
	if !ok {
		return "", errors.New("image path filter: not SequenceNode")
	}
	imageListNodes := containers.Values
	for i := range imageListNodes {
		t := imageListNodes[i].(*ast.StringNode)
		imageAndOldTag := strings.SplitN(t.Value, ":", 2)
		if imageAndOldTag[0] != image || imageAndOldTag[1] == tag {
			continue
		}

		t.Value = image + ":" + tag
		updated++
	}
	if updated == 0 {
		return "", ErrorImageNotFound
	}

	return astFile.String(), nil
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
