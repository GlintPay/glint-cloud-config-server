package api

import (
	"context"
	"reflect"
	"sort"
	"strings"

	"github.com/GlintPay/gccs/config"
	gotel "github.com/GlintPay/gccs/otel"
	"github.com/GlintPay/gccs/resolver/k8s"
	"github.com/GlintPay/gccs/utils"
	"github.com/rs/zerolog/log"
)

type Resolvable interface {
	ReconcileProperties(ctxt context.Context, applicationNames []string, profileNames []string, injections InjectedProperties, rawSource *Source) (ResolvedConfigValues, ResolutionMetadata, error)
}

type Resolver struct {
	flattenedStructure       bool
	templateConfig           config.GoTemplate
	enableTrace              bool
	pointlessOverrides       []duplicate
	k8sResolver              *k8s.Resolver
	propertiesResolverGetter func(context.Context, ResolvedConfigValues) PropertiesResolvable
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

	var listsToRemove []map[string]any
	if f.flattenedStructure {
		listsToRemove = findCompletelyReplacedFlattenedLists(rawSource.PropertySources)
	}

	for i, ps := range rawSource.PropertySources {
		for k, v := range ps.Source {

			if f.flattenedStructure && shouldSkipCompletelyReplacedFlattenedList(ps.Name, listsToRemove[i], k) {
				continue
			}

			f.overrideValue(reconciled, k, v, ps.Name)
		}
	}

	// Handle embedded references: ${propertyName} and ${propertyName:defaultValueIfMissing}. NB. Blank values don't trigger default.
	rr := f.newPropertiesResolverGetter(ctxt, applicationNames, profileNames, reconciled)
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

func (f *Resolver) overrideValue(reconciled map[string]any, k string, v any, source string) {
	if reconciled[k] == nil {
		reconciled[k] = v
		return
	}

	vKind := reflect.ValueOf(v).Kind()

	// Special treatment for Maps
	if vKind == reflect.Map {
		m := reconciled[k].(map[string]any)
		for ck, cv := range v.(map[string]any) {
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

func (f *Resolver) newPropertiesResolverGetter(ctx context.Context, applicationNames []string, profileNames []string, vals ResolvedConfigValues) PropertiesResolvable {
	if f.propertiesResolverGetter == nil {
		f.propertiesResolverGetter = func(ctx context.Context, r ResolvedConfigValues) PropertiesResolvable {
			return &PropertiesResolver{
				ctx:            ctx,
				data:           r,
				templateConfig: f.templateConfig.Validate(),
				templatesData: map[string]any{
					"Applications": applicationNames,
					"Profiles":     profileNames,
				},
				k8sResolver: f.k8sResolver,
			}
		}
	}

	return f.propertiesResolverGetter(ctx, vals)
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
