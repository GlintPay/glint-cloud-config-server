package file

import "github.com/GlintPay/gccs/config"

type Backend struct {
	Config config.FileConfig
}

func (b *Backend) Order() int {
	return b.Config.Order
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
