package git

import (
	"github.com/GlintPay/gccs/config"
	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Backend struct {
	Config      config.GitConfig
	Repo        *goGit.Repository
	PublicKeys  *ssh.PublicKeys
	EnableTrace bool
}

func (s *Backend) Order() int {
	return s.Config.Order
}

type fileItrWrapper struct {
	RepoUri string
	Files   *object.FileIter
	Dir     string
}

type fileWrapper struct {
	RepoUri string
	File    *object.File
	Dir     string
}

type fileBlob struct {
	Blob *object.Blob
}
