package file

import (
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
)

type Backend struct {
	Config    config.FileConfig
	Decrypter filetypes.Decrypter
}

func (s *Backend) Order() int {
	return s.Config.Order
}

type fileItrWrapper struct {
	DirPath   string
	Decrypter filetypes.Decrypter
}

type fileWrapper struct {
	FileName  string
	Path      string
	Dir       string
	Decrypter filetypes.Decrypter
}

type file struct {
	Path string
}
