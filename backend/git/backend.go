package git

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
	goGit "github.com/go-git/go-git/v5"
	goGitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	promApi "github.com/poblish/promenade/api"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Backend) Init(ctxt context.Context, config config.ApplicationConfiguration, metrics promApi.PrometheusMetrics) error {
	//fmt.Printf("config %+v\n", config)

	s.Config = config.Git
	s.Metrics = metrics

	hostKeyCallback, err := ssh.NewKnownHostsCallback(s.Config.KnownHostsFile)
	if err != nil {
		return err
	}

	s.PublicKeys, err = ssh.NewPublicKeys("git", []byte(strings.TrimSpace(s.Config.PrivateKey)), "")
	if err != nil {
		return err
	}

	s.PublicKeys.HostKeyCallback = hostKeyCallback

	if s.Config.CloneOnStart {
		fmt.Println("Clone on startup...")

		if e := s.connect(ctxt, "", !s.Config.DisableBaseDirCleaning); e != nil {
			return e
		}
	}

	return nil
}

func (s *Backend) connect(ctxt context.Context, branch string, cleanExisting bool) error {
	if s.Config.Basedir != "" && cleanExisting {
		fmt.Println("Cleaning existing...")

		err := os.RemoveAll(s.Config.Basedir)
		if err != nil {
			return err
		}
	}

	repo, err := goGit.PlainOpen(s.Config.Basedir)

	if err == goGit.ErrRepositoryNotExists {
		defer s.Metrics.Timer("clone")()

		depth := 0 // unlimited
		if s.Config.DisableLabels {
			depth = 1 // shallow
		}

		cloneOpts := &goGit.CloneOptions{
			Depth: depth,
			URL:   s.Config.Uri,
			Auth:  s.PublicKeys,
			//		Progress:  os.Stdout,
		}

		repo, err = goGit.PlainCloneContext(ctxt, s.Config.Basedir, false, cloneOpts)
		if err != nil {
			return err
		}

		fmt.Println("Cloned OK")
	} else if err != nil {
		return err
	} else {
		w, err := repo.Worktree()
		if err != nil {
			return err
		}

		if branch == "" {
			branch = s.Config.DefaultBranchName
			if branch == "" {
				branch = "master"
			}
		}

		ref := plumbing.ReferenceName("refs/heads/" + branch)

		coOpts := &goGit.CheckoutOptions{
			Branch: ref,
			Create: false,
			Force:  false,
			Keep:   false,
		}

		err = w.Checkout(coOpts)
		if err == nil {
			fmt.Printf("Checked out local [%s] OK\n", branch)
		} else {
			mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
			if err = fetchOrigin(repo, s.PublicKeys, mirrorRemoteBranchRefSpec); err != nil {
				return err
			}

			if err = w.Checkout(coOpts); err != nil {
				return err
			}

			fmt.Printf("Checked out remote [%s] OK\n", branch)
			return nil
		}

		err = w.Pull(&goGit.PullOptions{
			Depth:         1,
			ReferenceName: ref,
			Auth:          s.PublicKeys,
			//Progress: os.Stdout,
		})
		if err != nil && err != goGit.NoErrAlreadyUpToDate {
			return err
		}

		fmt.Println("Pulled OK")
	}

	s.Repo = repo

	return nil
}

// Borrowed from https://github.com/go-git/go-git/pull/446/files/62e512f0805303f9c245890bf2599295fc0f9774#diff-15808dd1f39f7d3198c9803a02fc1222b866ad5705b5aea887bb6a89ad572223
func fetchOrigin(repo *goGit.Repository, publicKeys *ssh.PublicKeys, refSpecStr string) error {
	remote, err := repo.Remote("origin")
	if err != nil {
		return err
	}

	var refSpecs []goGitConfig.RefSpec
	if refSpecStr != "" {
		refSpecs = []goGitConfig.RefSpec{goGitConfig.RefSpec(refSpecStr)}
	}

	if err = remote.Fetch(&goGit.FetchOptions{
		RefSpecs: refSpecs,
		Auth:     publicKeys,
	}); err != nil {
		if err == goGit.NoErrAlreadyUpToDate {
			fmt.Print("refs already up to date")
		} else {
			return fmt.Errorf("fetch origin failed: %v", err)
		}
	}

	return nil
}

func (s *Backend) GetCurrentState(ctxt context.Context, branch string, refresh bool) (*backend.State, error) {
	if refresh {
		if e := s.connect(ctxt, branch, false); e != nil {
			return nil, e
		}
	}

	ref, err := s.Repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := s.Repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	commitFiles, err := commit.Files()
	if err != nil {
		return nil, err
	}

	return &backend.State{
		Files:   fileItrWrapper{RepoUri: s.Config.Uri, Files: commitFiles},
		Version: commit.Hash.String(),
	}, nil
}

func (s *Backend) Close() {
	// NOOP
}

func (g fileWrapper) Name() string {
	return g.File.Name
}

func (g fileWrapper) IsReadable() (bool, string) {
	suffix := filepath.Ext(g.Name()) // FIXME need case-insensitivity?
	if suffix != ".yml" && suffix != ".yaml" {
		return false, ""
	}
	return true, suffix
}

func (g fileWrapper) ToMap() (map[string]interface{}, error) {
	return filetypes.FromYamlToMap(g)
}

func (g fileWrapper) FullyQualifiedName() string {
	return g.RepoUri + "/" + g.File.Name
}

func (g fileWrapper) Data() backend.Blob {
	return fileBlob{Blob: &g.File.Blob}
}

func (g fileBlob) Reader() (io.ReadCloser, error) {
	return g.Blob.Reader()
}

func (itr fileItrWrapper) ForEach(handler func(f backend.File) error) error {
	return itr.Files.ForEach(func(f *object.File) error {
		return handler(fileWrapper{RepoUri: itr.RepoUri, File: f})
	})
}
