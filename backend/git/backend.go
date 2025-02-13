package git

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codnect.io/chrono"
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
)

func (s *Backend) Init(ctxt context.Context, config config.ApplicationConfiguration) error {
	s.Config = config.Git

	s.YamlContext = filetypes.YamlContext{
		Decrypter: filetypes.SopsDecrypter{},
	}

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

	if s.Config.RefreshRateMillis > 0 {
		scheduler := chrono.NewDefaultTaskScheduler()

		period := time.Duration(s.Config.RefreshRateMillis) * time.Millisecond
		log.Info().Msgf("Scheduling pull every %v", period)

		_, err := scheduler.ScheduleAtFixedRate(func(ctx context.Context) {
			if e := s.connect(ctxt, "", false); e != nil {
				log.Error().Err(e).Msgf("Connect failed")
			}
		}, period)

		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Backend) connect(ctxt context.Context, branch string, cleanExisting bool) error {
	if cleanExisting {
		if e := s.cleanRepo(); e != nil {
			return e
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

		cloneOpts := s.getCloneOptions(ref, depth)
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

		head, err := repo.Head()
		if err == nil && head.Name() != ref {
			if err = s.checkout(ctxt, repo, w, branch, ref); err != nil {
				return err
			}
		}

		if s.Config.RefreshRateMillis <= 0 {
			if s.EnableTrace {
				_, span := gotel.GetTracer(ctxt).Start(ctxt, "git-pull", gotel.ServerOptions)
				defer span.End()
			}

			po := s.getPullOptions(ref)
			err = w.Pull(po)
			if err != nil && err != goGit.NoErrAlreadyUpToDate {
				return err
			}

			if s.Config.ForcePull {
				log.Debug().Msgf("Pulled OK (with force)")
			} else {
				log.Debug().Msgf("Pulled OK")
			}
		}
	}

	s.Repo = repo

	return nil
}

func (s *Backend) getCloneOptions(ref plumbing.ReferenceName, depth int) *goGit.CloneOptions {
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

	return cloneOpts
}

func (s *Backend) getPullOptions(ref plumbing.ReferenceName) *goGit.PullOptions {
	po := &goGit.PullOptions{
		ReferenceName: ref,
	}

	if s.PublicKeys != nil {
		po.Auth = s.PublicKeys
	}
	if s.Config.ShowProgress {
		po.Progress = os.Stdout
	}
	if s.Config.ForcePull {
		po.Force = true
	}

	return po
}

func (s *Backend) checkout(_ context.Context, repo *goGit.Repository, w *goGit.Worktree, branch string, ref plumbing.ReferenceName) error {
	coOpts := &goGit.CheckoutOptions{
		Branch: ref,
		Create: false,
		Force:  false,
		Keep:   false,
	}

	err := w.Checkout(coOpts)
	if err == nil {
		log.Debug().Msgf("Checked out local [%s] OK", branch)
		return nil
	}

	mirrorRemoteBranchRefSpec := fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch)
	if err = s.fetchOrigin(repo, mirrorRemoteBranchRefSpec); err != nil {
		return err
	}

	if err = w.Checkout(coOpts); err != nil {
		return err
	}

	log.Debug().Msgf("Checked out remote [%s] OK", branch)
	return nil
}

// Borrowed from https://github.com/go-git/go-git/pull/446/files/62e512f0805303f9c245890bf2599295fc0f9774#diff-15808dd1f39f7d3198c9803a02fc1222b866ad5705b5aea887bb6a89ad572223
func (s *Backend) fetchOrigin(repo *goGit.Repository, refSpecStr string) error {
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

	if s.Config.ShowProgress {
		fo.Progress = os.Stdout
	}
	if s.PublicKeys != nil {
		fo.Auth = s.PublicKeys
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

func (s *Backend) cleanRepo() error {
	if s.Config.Basedir == "" {
		return nil
	}
	log.Debug().Msg("Cleaning existing...")
	return os.RemoveAll(s.Config.Basedir)
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

	// Prevent `concurrent map writes` at `github.com/go-git/go-git/v5/plumbing/format/idxfile.(*MemoryIndex).genOffsetHash(0xc000262000)`
	s.commitsLock.Lock()
	defer s.commitsLock.Unlock()

	commit, err := s.Repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	commitFiles, err := commit.Files()
	if err != nil {
		return nil, err
	}

	return &backend.State{
		Files: fileItrWrapper{
			Dir:         s.Config.Basedir,
			RepoUri:     s.Config.Uri,
			Files:       commitFiles,
			YamlContext: s.YamlContext,
		},
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

func (g fileWrapper) ToMap() (map[string]any, error) {
	return filetypes.FromYamlToMap(g, g.YamlContext)
}

func (g fileWrapper) FullyQualifiedName() string {
	return g.RepoUri + "/" + g.File.Name
}

func (g fileWrapper) Location() string {
	return g.Dir
}

func (g fileWrapper) Data() backend.Blob {
	return fileBlob{Blob: &g.File.Blob}
}

func (g fileBlob) Reader() (io.ReadCloser, error) {
	return g.Blob.Reader()
}

func (itr fileItrWrapper) ForEach(handler func(f backend.File) error) error {
	return itr.Files.ForEach(func(f *object.File) error {
		return handler(fileWrapper{
			Dir:         itr.Dir,
			RepoUri:     itr.RepoUri,
			File:        f,
			YamlContext: itr.YamlContext,
		})
	})
}
