package git

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
	gotel "github.com/GlintPay/gccs/otel"
	goGit "github.com/go-git/go-git/v5"
	goGitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func (s *Backend) Init(ctxt context.Context, config config.ApplicationConfiguration) error {
	s.Config = config.Git

	if s.Config.PrivateKey != "" {
		hostKeyCallback, err := ssh.NewKnownHostsCallback(s.Config.KnownHostsFile)
		if err != nil {
			return err
		}

		s.PublicKeys, err = ssh.NewPublicKeys("git", []byte(strings.TrimSpace(s.Config.PrivateKey)), "")
		if err != nil {
			return err
		}

		s.PublicKeys.HostKeyCallback = hostKeyCallback
	}

	if s.Config.CloneOnStart {
		log.Debug().Msg("Clone on startup...")

		if e := s.connect(ctxt, "", !s.Config.DisableBaseDirCleaning); e != nil {
			return e
		}
	}

	return nil
}

func (s *Backend) connect(ctxt context.Context, branch string, cleanExisting bool) error {
	if s.Config.Basedir != "" && cleanExisting {
		log.Debug().Msg("Cleaning existing...")

		err := os.RemoveAll(s.Config.Basedir)
		if err != nil {
			return err
		}
	}

	repo, err := goGit.PlainOpen(s.Config.Basedir)

	if branch == "" {
		branch = s.Config.DefaultBranchName
		if branch == "" {
			branch = "master"
		}
	}
	ref := plumbing.ReferenceName("refs/heads/" + branch)

	if err == goGit.ErrRepositoryNotExists {
		if s.EnableTrace {
			_, span := gotel.GetTracer(ctxt).Start(ctxt, "git-clone", gotel.ServerOptions)
			defer span.End()
		}

		depth := 0 // unlimited
		if s.Config.DisableLabels {
			depth = 1 // shallow
		}

		cloneOpts := &goGit.CloneOptions{
			ReferenceName: ref,
			Depth:         depth,
			URL:           s.Config.Uri,
		}

		if s.PublicKeys != nil {
			cloneOpts.Auth = s.PublicKeys
		}
		if s.Config.ShowProgress {
			cloneOpts.Progress = os.Stdout
		}

		repo, err = goGit.PlainCloneContext(ctxt, s.Config.Basedir, false, cloneOpts)
		if err != nil {
			return err
		}

		log.Debug().Msgf("Cloned [%s] OK", branch)
	} else if err != nil {
		return err
	} else {
		w, err := repo.Worktree()
		if err != nil {
			return err
		}

		coOpts := &goGit.CheckoutOptions{
			Branch: ref,
			Create: false,
			Force:  false,
			Keep:   false,
		}

		err = w.Checkout(coOpts)
		if err == nil {
			log.Debug().Msgf("Checked out local [%s] OK", branch)
		} else {
			mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
			if err = fetchOrigin(repo, s.PublicKeys, mirrorRemoteBranchRefSpec); err != nil {
				return err
			}

			if err = w.Checkout(coOpts); err != nil {
				return err
			}

			log.Debug().Msgf("Checked out remote [%s] OK", branch)
			return nil
		}

		if s.EnableTrace {
			_, span := gotel.GetTracer(ctxt).Start(ctxt, "git-pull", gotel.ServerOptions)
			defer span.End()
		}

		po := &goGit.PullOptions{
			Depth:         1,
			ReferenceName: ref,
		}

		if s.PublicKeys != nil {
			po.Auth = s.PublicKeys
		}
		if s.Config.ShowProgress {
			po.Progress = os.Stdout
		}

		err = w.Pull(po)
		if err != nil && err != goGit.NoErrAlreadyUpToDate {
			return err
		}

		log.Debug().Msgf("Pulled OK")
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

	fo := &goGit.FetchOptions{
		RefSpecs: refSpecs,
	}

	if publicKeys != nil {
		fo.Auth = publicKeys
	}

	if err = remote.Fetch(fo); err != nil {
		if err == goGit.NoErrAlreadyUpToDate {
			log.Debug().Msgf("refs already up to date")
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
