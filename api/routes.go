package api

import (
	"encoding/json"
	"errors"
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/utils"
	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	applicationJSON = "application/json"
)

type Routing struct {
	ServerName   string
	ParentRouter chi.Router

	AppConfig config.ApplicationConfiguration
	Backends  backend.Backends

	resolverGetter func() Resolvable
}

func (rtr *Routing) SetupFunctionalRoutes(r chi.Router) error {
	if e := rtr.enableOTelForRouter(r); e != nil {
		return e
	}

	r.Get("/{application}/{profiles}", rtr.propertySourcesHandler())
	r.Get("/{application}/{profiles}/{labels}", rtr.propertySourcesHandler())
	r.Patch("/{application}/{profiles}", rtr.propertySourcesHandlerWithInjections())
	r.Patch("/{application}/{profiles}/{labels}", rtr.propertySourcesHandlerWithInjections())

	return nil
}

func (rtr *Routing) propertySourcesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		req, queries, err := rtr.newRequestFromChi(r)
		if err != nil {
			rtr.writeError(w, err)
			return
		}

		source, err := LoadConfigurations(r.Context(), rtr.Backends, req)
		if err != nil {
			rtr.writeError(w, err)
			return
		}

		var configJSONBytes []byte
		var outputErr error

		resolveVal := overrideBooleanDefault(queries.Get("resolve"), rtr.AppConfig.Defaults.ResolvePropertySources)
		if resolveVal {
			resolver := rtr.newResolver(req)
			values, metadata, e := resolver.ReconcileProperties(r.Context(), req.Applications, req.Profiles, InjectedProperties{}, source)
			if e != nil {
				rtr.writeError(w, e)
				return
			}

			writeHeaders(w.Header(), req, metadata, source)

			configJSONBytes, outputErr = marshalResponseJSON(values, req.PrettyPrintJson)
		} else {
			configJSONBytes, outputErr = marshalResponseJSON(source, req.PrettyPrintJson)
		}

		rtr.handleOutput(w, outputErr, configJSONBytes, req.LogResponses)
	}
}

func (rtr *Routing) propertySourcesHandlerWithInjections() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		req, queries, err := rtr.newRequestFromChi(r)
		if err != nil {
			rtr.writeError(w, err)
			return
		}

		source, err := LoadConfigurations(r.Context(), rtr.Backends, req)
		if err != nil {
			rtr.writeError(w, err)
			return
		}

		var configJSONBytes []byte
		var outputErr error

		resolveVal := overrideBooleanDefault(queries.Get("resolve"), rtr.AppConfig.Defaults.ResolvePropertySources)
		if resolveVal {
			injected := InjectedProperties{}

			bs, _ := io.ReadAll(r.Body)
			if len(bs) > 0 {
				err = json.Unmarshal(bs, &injected)
				if err != nil {
					rtr.writeError(w, errors.New("Unparseable JSON: "+err.Error()))
					return
				}
			}

			resolver := rtr.newResolver(req)
			values, metadata, e := resolver.ReconcileProperties(r.Context(), req.Applications, req.Profiles, injected, source)
			if e != nil {
				rtr.writeError(w, e)
				return
			}

			writeHeaders(w.Header(), req, metadata, source)

			configJSONBytes, outputErr = marshalResponseJSON(values, req.PrettyPrintJson)
		} else {
			configJSONBytes, outputErr = marshalResponseJSON(source, req.PrettyPrintJson)
		}

		rtr.handleOutput(w, outputErr, configJSONBytes, req.LogResponses)
	}
}

func marshalResponseJSON(val interface{}, pretty bool) ([]byte, error) {
	if pretty {
		return json.MarshalIndent(val, "", "  ")
	}
	return json.Marshal(val)
}

func writeHeaders(header http.Header, req ConfigurationRequest, metadata ResolutionMetadata, source *Source) {
	header.Set("X-Resolution-PrecedenceDisplayMessage", metadata.PrecedenceDisplayMessage)
	header.Set("X-Resolution-Name", strings.Join(req.Applications, ","))
	header.Set("X-Resolution-Profiles", strings.Join(req.Profiles, ","))
	header.Set("X-Resolution-Label", "")
	header.Set("X-Resolution-Version", source.Version)
}

func (rtr *Routing) handleOutput(w http.ResponseWriter, err error, bytes []byte, logResponses bool) {
	if err != nil {
		rtr.writeError(w, err)
		return
	}

	w.Header().Set("Content-Type", applicationJSON)
	_, _ = w.Write(bytes)

	if logResponses {
		log.Debug().Msgf("Response: %s", string(bytes))
	}
}

func (rtr *Routing) writeError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)

	info := map[string]interface{}{"message": err.Error()}
	_ = json.NewEncoder(w).Encode(info)

	log.Error().Err(err).Stack().Msg("Response error")
}

func (rtr *Routing) newRequestFromChi(r *http.Request) (ConfigurationRequest, url.Values, error) {
	matchApplicationCsv := chi.URLParam(r, "application")
	matchProfilesCsv := chi.URLParam(r, "profiles")

	labels := chi.URLParam(r, "labels")
	if rtr.AppConfig.Git.DisableLabels && labels != "" {
		return ConfigurationRequest{}, nil, errors.New("cannot specify a label when `git.disableLabels` is true")
	}

	queries := r.URL.Query()

	flattenVal := overrideBooleanDefault(queries.Get("flatten"), rtr.AppConfig.Defaults.FlattenHierarchicalConfig)
	flattenedIndexedListsVal := overrideBooleanDefault(queries.Get("flattenLists"), rtr.AppConfig.Defaults.FlattenedIndexedLists)
	logResponses := overrideBooleanDefault(queries.Get("logResponses"), rtr.AppConfig.Defaults.LogResponses)
	prettyPrintJSON := overrideBooleanDefault(queries.Get("pretty"), rtr.AppConfig.Defaults.PrettyPrintJson)

	return ConfigurationRequest{
		Applications: utils.SplitApplicationNames(matchApplicationCsv),
		Profiles:     utils.SplitProfileNames(matchProfilesCsv),
		Labels:       LabelsRequest{Branch: labels},

		RefreshBackend:        !queries.Has("norefresh"),
		FlattenHierarchies:    flattenVal,
		FlattenedIndexedLists: flattenedIndexedListsVal,
		LogResponses:          logResponses,
		PrettyPrintJson:       prettyPrintJSON,

		EnableTrace: rtr.AppConfig.Tracing.Enabled,
	}, queries, nil
}

func (rtr *Routing) enableOTelForRouter(r chi.Router) error {
	if !rtr.AppConfig.Tracing.Enabled {
		return nil
	}

	if rtr.ServerName == "" || rtr.ParentRouter == nil {
		return errors.New("OTel not configured")
	}

	r.Use(otelchi.Middleware(rtr.ServerName, otelchi.WithChiRoutes(rtr.ParentRouter)))

	log.Info().Msgf("OpenTelemetry trace is enabled")
	return nil
}

func (rtr *Routing) newResolver(req ConfigurationRequest) Resolvable {
	if rtr.resolverGetter == nil {
		rtr.resolverGetter = func() Resolvable {
			return &Resolver{
				flattenedStructure: req.FlattenedIndexedLists,
				templateConfig:     rtr.AppConfig.Gotemplate,
				enableTrace:        rtr.AppConfig.Tracing.Enabled,
			}
		}
	}

	return rtr.resolverGetter()
}

func overrideBooleanDefault(queryValue string, defaultVal bool) bool {
	reqVal := strings.ToLower(queryValue)
	if reqVal == "true" {
		return true
	} else if reqVal == "false" {
		return false
	}
	return defaultVal
}
