package health

import (
	"github.com/go-chi/chi/v5"
)

type opts struct {
	ChiMux *chi.Mux
}

type Opt func(*opts)

func WithChiMux(mux *chi.Mux) Opt {
	return func(o *opts) {
		o.ChiMux = mux
	}
}
