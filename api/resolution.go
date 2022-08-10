package api

import (
	"context"
	gotel "github.com/GlintPay/gccs/otel"
	"github.com/GlintPay/gccs/utils"
	"github.com/rs/zerolog/log"
	"reflect"
	"sort"
	"strings"
)

type Resolvable interface {
	ReconcileProperties(ctxt context.Context, applicationNames []string, profileNames []string, injections InjectedProperties, rawSource *Source) (ResolvedConfigValues, ResolutionMetadata, error)
}

type Resolver struct {
	enableTrace              bool
	pointlessOverrides       []duplicate
	propertiesResolverGetter func(ResolvedConfigValues) PropertiesResolvable
}

func (f *Resolver) ReconcileProperties(ctxt context.Context, applicationNames []string, profileNames []string, injections InjectedProperties, rawSource *Source) (ResolvedConfigValues, ResolutionMetadata, error) {
	if f.enableTrace {
		_, span := gotel.GetTracer(ctxt).Start(ctxt, "reconcile", gotel.ServerOptions)
		defer span.End()
	}

	reconciled := make(ResolvedConfigValues)

	// Copy ^ ones at lowest level
	for k, v := range injections {
		if preprocess(k) {
			key := k[1:]
			f.overrideValue(reconciled, key, v, "preprocess")
		}
	}

	// Deterministic sorting, regardless of any other implicit ordering
	sorter := Sorter{AppNames: applicationNames, Profiles: profileNames, Sources: rawSource.PropertySources}
	sort.SliceStable(rawSource.PropertySources, sorter.Sort())

	sourceNames := getPropertySourceNames(rawSource.PropertySources)

	for _, ps := range rawSource.PropertySources {
		for k, v := range ps.Source {
			f.overrideValue(reconciled, k, v, ps.Name)
		}
	}

	// Handle embedded references: ${propertyName} and ${propertyName:defaultValueIfMissing}. NB. Blank values don't trigger default.
	rr := f.newPropertiesResolverGetter(reconciled)
	if _, e := rr.resolvePlaceholdersFromTop(); e != nil {
		return reconciled, ResolutionMetadata{}, e
	}

	// Copy non-^ ones at highest level
	for k, v := range injections {
		if postprocess(k) {
			f.overrideValue(reconciled, k, v, "postprocess")
		}
	}

	if len(f.pointlessOverrides) > 0 {
		log.Info().Msgf("Unnecessary overrides were found: %v", f.pointlessOverrides)
	}

	return reconciled, ResolutionMetadata{
		PrecedenceDisplayMessage: sourceNames,
	}, nil
}

func (f *Resolver) overrideValue(reconciled map[string]interface{}, k string, v interface{}, source string) {
	if reconciled[k] == nil {
		reconciled[k] = v
		return
	}

	vKind := reflect.ValueOf(v).Kind()

	// Special treatment for Maps
	if vKind == reflect.Map {
		m := reconciled[k].(map[string]interface{})
		for ck, cv := range v.(map[string]interface{}) {
			m[ck] = cv
		}
		return
	}

	// Special treatment for Slices
	if vKind == reflect.Slice {
		// Completely replace the list, don't merge it
		reconciled[k] = v
		return
	}

	// For any other types...
	currValue := reconciled[k]
	if currValue != v {
		reconciled[k] = v
	} else {
		f.pointlessOverrides = append(f.pointlessOverrides, duplicate{key: k, value: v, source: source})
	}
}

func (f *Resolver) newPropertiesResolverGetter(vals ResolvedConfigValues) PropertiesResolvable {
	if f.propertiesResolverGetter == nil {
		f.propertiesResolverGetter = func(r ResolvedConfigValues) PropertiesResolvable {
			return &PropertiesResolver{data: r}
		}
	}

	return f.propertiesResolverGetter(vals)
}

func getPropertySourceNames(sources []PropertySource) string {
	if len(sources) < 1 {
		return ""
	}

	var s []string
	for i := len(sources) - 1; i >= 0; i-- { // reverse order
		s = append(s, utils.StripGitPrefix(sources[i].Name))
	}
	return strings.Join(s, " > ")
}
