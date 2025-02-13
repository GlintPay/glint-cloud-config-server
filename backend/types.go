package backend

import (
	"context"
	"github.com/GlintPay/gccs/config"
	"io"
)

type Backends []Backend

type Backend interface {
	Ordering
	Init(ctxt context.Context, config config.ApplicationConfiguration) error
	GetCurrentState(ctxt context.Context, branch string, refresh bool) (*State, error)
	Close()
}

type Ordering interface {
	Order() int // lower is higher priority
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
	Location() string

	IsReadable() (bool, string)
	Data() Blob
	ToMap() (map[string]any, error)
}

type Blob interface {
	Reader() (io.ReadCloser, error)
}
