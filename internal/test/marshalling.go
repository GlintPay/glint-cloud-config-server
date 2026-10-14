package test

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/shopspring/decimal"
	"github.com/wolfeidau/unflatten"
	"strings"
	"time"
)

const DecimalSuffix = "_Decimal"
const DurationSuffix = "_Duration"
const StringSuffix = "_String"

// MarshalFlattenedTo Marshal a map of properties to a result structure without unflattening any
// hierarchical property names, e.g. `service.host: foo` and `service.port: 123` are not combined
// under a common parent. `mapstructure` metadata may be required for this to work.
func MarshalFlattenedTo(v map[string]any, outputStruct any) error {
	return marshalTo(v, outputStruct)
}

// MarshalHierarchicalTo Marshal a map of properties to a result structure, restructuring
// hierarchical properties, such that `service.host: foo` and `service.port: 123` are grouped
// under a common parent. This may require a set of nested structures to be defined.
func MarshalHierarchicalTo(v map[string]any, outputStruct any) error {
	return marshalTo(unflatten.Unflatten(v, func(k string) []string { return strings.Split(k, ".") }), outputStruct)
}

func marshalTo(source map[string]any, outputStruct any) error {
	if err := handleFlattenedLists(source); err != nil {
		return err
	}

	config := &mapstructure.DecoderConfig{Metadata: nil, ZeroFields: true, TagName: "from", Result: outputStruct}
	decoder, _ := mapstructure.NewDecoder(config)

	return decoder.Decode(source)
}

func handleFlattenedLists(source map[string]any) error {
	if err := remapDataValues(source); err != nil {
		return err
	}
	return nil
}

func remapDataValues(source map[string]any) error {
	var overallErr error = nil

	// Map data items recursively - but don't restructure any lists or structures
	for k, propertyValue := range source {
		if err := remapDataValue(source, k, propertyValue); err != nil {
			return err
		}

		// Recurse into value maps...
		if propertyValueTyped, ok := propertyValue.(map[string]any); ok {
			if e := remapDataValues(propertyValueTyped); e != nil {
				overallErr = e
			}
		}
	}

	return overallErr
}

func remapDataValue(source map[string]any, k string, propertyValue any) error {
	switch {
	case strings.HasSuffix(k, "CSV"): // Properties ending in "CSV" will, as a convenience, be split into a list on the fly
		list := strings.Split(propertyValue.(string), ",")
		if len(list) > 0 {
			for i := range list {
				list[i] = strings.TrimSpace(list[i])
			}

			source[k[:len(k)-3]] = list
		}
	case strings.Index(k, DurationSuffix) > 0:
		return remapParsed(source, k, DurationSuffix, propertyValue, time.ParseDuration)
	case strings.Index(k, DecimalSuffix) > 0: // Properties ending in DecimalSuffix will be converted from string to decimal
		return remapParsed(source, k, DecimalSuffix, propertyValue, decimal.NewFromString)
	case strings.Index(k, StringSuffix) > 0: // Currently only to support overrides that looked like numbers but are meant as strings
		propertyValueTyped, ok := propertyValue.(string)
		if !ok {
			return fmt.Errorf("unexpected value type %+v - should be string", propertyValue)
		}
		source[strings.Replace(k, StringSuffix, "", 1)] = propertyValueTyped
		delete(source, k)
	}
	return nil
}

func remapParsed[T any](source map[string]any, k string, suffix string, propertyValue any, parse func(string) (T, error)) error {
	switch propertyValueTyped := propertyValue.(type) {
	case string:
		parsed, err := parse(propertyValueTyped)
		if err == nil {
			source[strings.Replace(k, suffix, "", 1)] = parsed
			delete(source, k)
		}
	case map[string]any:
		for pkey, pval := range propertyValueTyped {
			parsed, err := parse(pval.(string))
			if err == nil {
				propertyValueTyped[pkey] = parsed
			}
		}
		source[strings.Replace(k, suffix, "", 1)] = propertyValueTyped
		delete(source, k)
	default:
		return fmt.Errorf("unexpected value type %+v - should be string or map[string]any", propertyValueTyped)
	}
	return nil
}
