package file

import (
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
)

type Backend struct {
	Config      config.FileConfig
	YamlContext filetypes.YamlContext
}

func (s *Backend) Order() int {
	return s.Config.Order
}

type fileItrWrapper struct {
	DirPath     string
	YamlContext filetypes.YamlContext
}

type fileWrapper struct {
	FileName    string
	Path        string
	Dir         string
	YamlContext filetypes.YamlContext
}

type file struct {
	Path string
}
