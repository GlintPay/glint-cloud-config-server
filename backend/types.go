package backend

import (
	"context"
	"github.com/GlintPay/gccs/config"
	promApi "github.com/poblish/promenade/api"
	"io"
)

type Backends []Backend

type Backend interface {
	Init(ctxt context.Context, config config.ApplicationConfiguration, metrics promApi.PrometheusMetrics) error
	GetCurrentState(ctxt context.Context, branch string, refresh bool) (*State, error)
	Close()
}

type State struct {
	Version string
	Files   FileIterator
}

type FileIterator interface {
	ForEach(f func(f File) error) error
}

type File interface {
	Name() string
	FullyQualifiedName() string

	IsReadable() (bool, string)
	Data() Blob
	ToMap() (map[string]interface{}, error)
}

type Blob interface {
	Reader() (io.ReadCloser, error)
}
