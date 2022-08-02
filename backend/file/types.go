package file

import (
	promApi "github.com/poblish/promenade/api"
)

type Backend struct {
	DirPath string
	Metrics promApi.PrometheusMetrics
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
