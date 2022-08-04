package file

import (
	"context"
	"errors"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"path"
	"path/filepath"
)

func (s *Backend) Init(_ context.Context, appConfig config.ApplicationConfiguration) error {
	s.Config = appConfig.File
	log.Debug().Msgf("Reading from %s", s.Config.Path)
	return nil
}

func (s *Backend) GetCurrentState(_ context.Context, branch string, _ bool) (*backend.State, error) {
	if branch != "" {
		return nil, errors.New("labels, multiple branches not supported by File backend")
	}

	return &backend.State{
		Files:   fileItrWrapper{DirPath: s.Config.Path},
		Version: "",
	}, nil
}

func (s *Backend) Close() {
	// NOOP
}

func (g fileWrapper) Name() string {
	return g.FileName
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
	return g.Path
}

func (g fileWrapper) Data() backend.Blob {
	return file{Path: g.Path}
}

func (g file) Reader() (io.ReadCloser, error) {
	return os.Open(g.Path)
}

func (itr fileItrWrapper) ForEach(handler func(f backend.File) error) error {
	dirEntry, err := os.ReadDir(itr.DirPath)
	if err != nil {
		return err
	}

	for _, d := range dirEntry {
		name := d.Name()
		filePath := path.Join([]string{itr.DirPath, name}...)
		if e := handler(fileWrapper{FileName: name, Path: filePath}); e != nil {
			return e
		}
	}
	return nil
}
