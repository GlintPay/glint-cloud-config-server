package health

import (
	"github.com/heptiolabs/healthcheck"
	"net/http"
)

// New - A liveness check indicates that this instance of the application should be destroyed and replaced. A failed liveness check
// indicates that this instance is unhealthy, not some upstream dependency.
//
// A readiness Check indicates that this instance of the application is currently unable to serve requests because of an upstream
// or some transient failure. Readiness includes all liveness checks, and is their superset.
//
// Liveness and Readiness endpoints will return 200 / OK out of the box to represent broad application health
// They will start to return 5xx only if new healthchecks are added and only if those start to error
//
func New(opts ...Opt) *Healthchecks {
	facade := &Healthchecks{handler: healthcheck.NewHandler()}

	for _, optionFunc := range opts {
		optionFunc(&facade.opts)
	}

	return facade
}

type Healthchecks struct {
	opts
	handler healthcheck.Handler
}

// StartListening Start the endpoints once we believe that we are broadly healthy
func (f *Healthchecks) StartListening() {
	if f.ChiMux != nil {
		f.ChiMux.Handle("/liveness", http.HandlerFunc(f.handler.LiveEndpoint))
		f.ChiMux.Handle("/readiness", http.HandlerFunc(f.handler.ReadyEndpoint))
	}
}
