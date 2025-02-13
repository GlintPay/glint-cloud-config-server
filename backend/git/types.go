package git

import (
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
	goGit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"sync"
)

type Backend struct {
	Config      config.GitConfig
	Repo        *goGit.Repository
	PublicKeys  *ssh.PublicKeys
	EnableTrace bool

	commitsLock sync.RWMutex

	Decrypter filetypes.Decrypter
}

func (s *Backend) Order() int {
	return s.Config.Order
}

type fileItrWrapper struct {
	RepoUri   string
	Files     *object.FileIter
	Dir       string
	Decrypter filetypes.Decrypter
}

type fileWrapper struct {
	RepoUri   string
	File      *object.File
	Dir       string
	Decrypter filetypes.Decrypter
}

type fileBlob struct {
	Blob *object.Blob
}
