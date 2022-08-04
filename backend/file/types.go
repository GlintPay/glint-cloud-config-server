package file

import "github.com/GlintPay/gccs/config"

type Backend struct {
	Config config.FileConfig
}

type fileItrWrapper struct {
	DirPath string
}

type fileWrapper struct {
	FileName string
	Path     string
}

type file struct {
	Path string
}
