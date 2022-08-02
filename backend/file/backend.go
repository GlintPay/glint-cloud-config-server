package file

import (
	"context"
	"errors"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/filetypes"
	promApi "github.com/poblish/promenade/api"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

func (s *Backend) Init(_ context.Context, _ config.ApplicationConfiguration, metrics promApi.PrometheusMetrics) error {
	s.Metrics = metrics
	return nil
}

func (s *Backend) GetCurrentState(_ context.Context, branch string, _ bool) (*backend.State, error) {
	if branch != "" {
		return nil, errors.New("labels, multiple branches not supported by File backend")
	}

	return &backend.State{
		Files:   fileItrWrapper{DirPath: s.DirPath},
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
	fileInfo, err := ioutil.ReadDir(itr.DirPath)
	if err != nil {
		return err
	}

	for _, d := range fileInfo {
		name := d.Name()
		filePath := path.Join([]string{itr.DirPath, name}...)
		if e := handler(fileWrapper{FileName: name, Path: filePath}); e != nil {
			return e
		}
	}
	return nil
}
