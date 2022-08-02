package api

import (
	"context"
	"fmt"
	gotel "github.com/GlintPay/gccs/otel"
	"github.com/GlintPay/gccs/utils"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type Resolver struct {
	enableTrace        bool
	pointlessOverrides []duplicate
}

var placeholderRegex = regexp.MustCompile(`\${([^}]*)}`)

func (f *Resolver) ReconcileProperties(ctxt context.Context, applicationNames []string, profileNames []string, injections InjectedProperties, rawSource *Source) (ResolvedConfigValues, ResolutionMetadata) {
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
	resolvePlaceholders(reconciled, reconciled)

	// Copy non-^ ones at highest level
	for k, v := range injections {
		if postprocess(k) {
			f.overrideValue(reconciled, k, v, "postprocess")
		}
	}

	if len(f.pointlessOverrides) > 0 {
		fmt.Println("Unnecessary overrides were found:", f.pointlessOverrides)
	}

	return reconciled, ResolutionMetadata{
		PrecedenceDisplayMessage: sourceNames,
	}
}

func resolvePlaceholders(allValues ResolvedConfigValues, currentMap map[string]interface{}) {
	for propertyName, v := range currentMap {
		switch typedVal := v.(type) {
		case map[string]interface{}:
			resolvePlaceholders(allValues, typedVal)
		case []string:
			// FIXME Incomplete
			//for propertyName, v := range reconciled {
			//	resolvePlaceholders(typedVal)
			//}
		case string:
			currentMap[propertyName] = resolveString(allValues, propertyName, typedVal)
		}
	}
}

// FIXME Should missing properties be a configurable fatal error?
func resolveString(allValues ResolvedConfigValues, propertyName string, value string) string {
	// Don't check for pointless overrides here, it's expected
	return placeholderRegex.ReplaceAllStringFunc(value, func(foundMatch string) string {
		sourcePropertyWithDefault := strings.Split(strings.TrimSpace(foundMatch[2:len(foundMatch)-1]), ":")

		if sourcePropertyWithDefault[0] != "" {
			if resolvedPropertyValue, ok := allValues[sourcePropertyWithDefault[0]]; ok {
				// Found a match, replace with `resolvedPropertyValue`
				return resolvedPropertyValue.(string)
			} else if len(sourcePropertyWithDefault) < 2 {
				// No match, no default
				fmt.Printf("Missing value for property [%s]\n", sourcePropertyWithDefault[0])
			} else if len(sourcePropertyWithDefault) > 1 {
				// No match, use available default
				return sourcePropertyWithDefault[1]
			}
		}

		// ${} is not acceptable
		fmt.Printf("Missing placeholder [%s] for property [%s]\n", foundMatch, propertyName)
		return ""
	})
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
		// fmt.Println(k, "... was ", m)
		for ck, cv := range v.(map[string]interface{}) {
			m[ck] = cv
		}
		// fmt.Println(k, "...", m)
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
