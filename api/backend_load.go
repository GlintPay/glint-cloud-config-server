package api

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/backend"
	gotel "github.com/GlintPay/gccs/otel"
	"github.com/GlintPay/gccs/utils"
	"strings"
)

func LoadConfiguration(ctxt context.Context, s backend.Backend, req ConfigurationRequest) (*Source, error) {
	fmt.Printf("Requesting: %s/%s/[%s]\n", req.Applications, req.Profiles, req.Labels)

	if req.EnableTrace {
		_, span := gotel.GetTracer(ctxt).Start(ctxt, "loadConfiguration", gotel.ServerOptions)
		defer span.End()
	}

	state, err := s.GetCurrentState(ctxt, req.Labels.Branch, req.RefreshBackend)
	if err != nil {
		return nil, err
	}

	sourceName := ""
	if len(req.Applications) > 0 { // TODO Validate higher up?
		sourceName = req.Applications[0]
	}

	source := Source{
		Name:            sourceName,
		Profiles:        req.Profiles,
		Label:           "",
		State:           "",
		Version:         state.Version,
		PropertySources: make([]PropertySource, 0),
	}

	if state.Files == nil {
		return &source, nil
	}

	addHandler := newDiscoveryHandler(req, &source)

	/* https://docs.spring.io/spring-cloud-config/docs/current/reference/html/#_quick_start
	The HTTP service has resources in the form:

		/{application}/{profile}[/{label}]
		/{application}-{profile}.yml
		/{label}/{application}-{profile}.yml
		/{application}-{profile}.properties
		/{label}/{application}-{profile}.properties

	"label" is an optional git label (defaults to "master".)
	*/
	err = state.Files.ForEach(func(f backend.File) error {
		readable, suffix := f.IsReadable()
		if !readable {
			return nil
		}

		filename := strings.TrimSuffix(f.Name(), suffix)

		// Always add application.yml
		if filename == utils.DefaultApplicationName {
			return addHandler(f)
		}

		// Add any matching application-{profile}.yml
		if len(req.Profiles) > 0 && strings.HasPrefix(filename, utils.DefaultApplicationNamePrefix) {
			return findAmongProfiles(f, filename, utils.DefaultApplicationNamePrefix, req.Profiles, addHandler)
		}

		// For each {application}...
		for _, eachWantedApplication := range req.Applications {
			if eachWantedApplication == utils.DefaultApplicationName {
				continue
			}

			// Add any matching {application}.yml
			if filename == eachWantedApplication {
				if e := addHandler(f); e != nil {
					return e
				}
			} else if strings.HasPrefix(filename, eachWantedApplication+"-") {
				// Add any matching {application}-{profile}.yml
				if e := findAmongProfiles(f, filename, eachWantedApplication+"-", req.Profiles, addHandler); e != nil {
					return e
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &source, nil
}

func findAmongProfiles(f backend.File, filename string, profile string, wantedProfiles []string, handler discoveryHandler) error {
	profileFound := filename[len(profile):]
	for _, eachWantedProfile := range wantedProfiles {
		if profileFound == eachWantedProfile {
			err := handler(f)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

type discoveryHandler func(f backend.File) error

var joinerFunc = func(k []string) string {
	return strings.Join(k, ".")
}

func newDiscoveryHandler(req ConfigurationRequest, source *Source) discoveryHandler {
	return func(f backend.File) error {
		// fmt.Println("Discovered", f.Name)

		mapStructuredData, err := f.ToMap()
		if err != nil {
			return err
		}

		if req.FlattenHierarchies {
			mapStructuredData = utils.Flatten(mapStructuredData, joinerFunc)

			if req.FlattenedIndexedLists {
				flattenedIndexedLists(mapStructuredData)

				// Reflatten just in case
				mapStructuredData = utils.Flatten(mapStructuredData, joinerFunc)
			}
		}

		ps := PropertySource{
			Name:   f.FullyQualifiedName(),
			Source: mapStructuredData,
		}

		source.PropertySources = append(source.PropertySources, ps)
		return nil
	}
}

func flattenedIndexedLists(data map[string]interface{}) {
	for k, v := range data {
		switch typed := v.(type) {
		case []interface{}:
			{
				// FIXME Empty list?
				for i, val := range typed {
					newKey := fmt.Sprintf("%s[%d]", k, i)
					data[newKey] = val

					switch typedMap := val.(type) {
					case map[string]interface{}:
						flattenedIndexedLists(typedMap)
					}
				}
				delete(data, k) // remove original
			}
		}
	}
}
