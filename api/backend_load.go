package api

import (
	"context"
	"fmt"
	"github.com/GlintPay/gccs/backend"
	gotel "github.com/GlintPay/gccs/otel"
	"github.com/GlintPay/gccs/utils"
	"github.com/rs/zerolog/log"
	"sort"
	"strings"
)

func LoadConfigurations(ctxt context.Context, s backend.Backends, req ConfigurationRequest) (*Source, error) {
	sorter := backend.Sorter{Backends: s}
	sort.SliceStable(s, sorter.Sort())

	sourceName := ""
	if len(req.Applications) > 0 { // TODO Validate higher up?
		sourceName = req.Applications[0]
	}

	source := &Source{
		Name:            sourceName,
		Profiles:        req.Profiles,
		Label:           "",
		State:           "",
		Version:         "",
		PropertySources: make([]PropertySource, 0),
	}

	for _, each := range s {
		if e := loadConfiguration(ctxt, each, req, source); e != nil {
			return &Source{}, e
		}
	}
	return source, nil
}

func loadConfiguration(ctxt context.Context, s backend.Backend, req ConfigurationRequest, source *Source) error {
	// log.Debug().Msgf("Requesting: %s/%s/[%s]", req.Applications, req.Profiles, req.Labels)

	if req.EnableTrace {
		_, span := gotel.GetTracer(ctxt).Start(ctxt, "loadConfiguration", gotel.ServerOptions)
		defer span.End()
	}

	state, err := s.GetCurrentState(ctxt, req.Labels.Branch, req.RefreshBackend)
	if err != nil {
		return err
	}

	// Join new version to existing, FWIW
	if len(state.Version) > 0 {
		if len(source.Version) > 0 {
			source.Version += "; "
		}
		source.Version += state.Version
	}

	////////////////////////////////////////////////////

	addHandler := newDiscoveryHandler(req, source)

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
		return err
	}

	return nil
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
		log.Info().Msgf("Adding property source: Config resource '%s' via location '%s'", f.FullyQualifiedName(), f.Location())

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
		//fmt.Println("==>", ps)

		source.PropertySources = append(source.PropertySources, ps)
		return nil
	}
}

func flattenedIndexedLists(data map[string]any) {
	// First pass: collect all arrays to flatten
	type arrayToFlatten struct {
		key   string
		array []any
	}
	var arraysToFlatten []arrayToFlatten

	for k, v := range data {
		if arr, ok := v.([]any); ok {
			arraysToFlatten = append(arraysToFlatten, arrayToFlatten{key: k, array: arr})
		}
	}
	fmt.Println("==>", arraysToFlatten)

	// Second pass: flatten all collected arrays
	var mapsToProcess []map[string]any

	for _, item := range arraysToFlatten {
		k := item.key
		arr := item.array

		// Handle each element in the array
		for i, val := range arr {
			newKey := fmt.Sprintf("%s[%d]", k, i)
			data[newKey] = val

			// Collect nested maps for later processing
			if nestedMap, ok := val.(map[string]any); ok {
				mapsToProcess = append(mapsToProcess, nestedMap)
			}
		}

		// Remove the original array
		delete(data, k)
	}

	// Final pass: process all nested maps we collected
	for _, nestedMap := range mapsToProcess {
		flattenedIndexedLists(nestedMap)
	}
}
