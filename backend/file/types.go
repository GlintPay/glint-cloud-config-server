package file

import "github.com/GlintPay/gccs/config"

type Backend struct {
	Config config.FileConfig
}

func (s *Backend) Order() int {
	return s.Config.Order
}

type fileItrWrapper struct {
	DirPath string
}

type fileWrapper struct {
	FileName string
	Path     string
	Dir      string
}

type file struct {
	Path string
}
