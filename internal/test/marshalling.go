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

func remapDataValues(source map[string]any) error { //nolint
	var overallErr error = nil

	// Map data items recursively - but don't restructure any lists or structures
	for k, propertyValue := range source {
		if strings.HasSuffix(k, "CSV") { // Properties ending in "CSV" will, as a convenience, be split into a list on the fly
			list := strings.Split(propertyValue.(string), ",")
			if len(list) > 0 {
				for i := range list {
					list[i] = strings.TrimSpace(list[i])
				}

				source[k[:len(k)-3]] = list
			}
		} else if strings.Index(k, DurationSuffix) > 0 {
			switch propertyValueTyped := propertyValue.(type) {
			case string:
				durationVal, err := time.ParseDuration(propertyValueTyped)
				if err == nil {
					source[strings.Replace(k, DurationSuffix, "", 1)] = durationVal
					delete(source, k)
				}
			case map[string]any:
				for pkey, pval := range propertyValueTyped {
					decValue, err := time.ParseDuration(pval.(string))
					if err == nil {
						propertyValueTyped[pkey] = decValue
					}
				}
				source[strings.Replace(k, DurationSuffix, "", 1)] = propertyValueTyped
				delete(source, k)
			default:
				return fmt.Errorf("unexpected value type %+v - should be string or map[string]any", propertyValueTyped)
			}
		} else if strings.Index(k, DecimalSuffix) > 0 { // Properties ending in DecimalSuffix will be converted from string to decimal
			switch propertyValueTyped := propertyValue.(type) {
			case string:
				decValue, err := decimal.NewFromString(propertyValueTyped)
				if err == nil {
					source[strings.Replace(k, DecimalSuffix, "", 1)] = decValue
					delete(source, k)
				}
			case map[string]any:
				for pkey, pval := range propertyValueTyped {
					decValue, err := decimal.NewFromString(pval.(string))
					if err == nil {
						propertyValueTyped[pkey] = decValue
					}
				}
				source[strings.Replace(k, DecimalSuffix, "", 1)] = propertyValueTyped
				delete(source, k)
			default:
				return fmt.Errorf("unexpected value type %+v - should be string or map[string]any", propertyValueTyped)
			}
		} else if strings.Index(k, StringSuffix) > 0 { // Currently only to support overrides that looked like numbers but are meant as strings
			switch propertyValueTyped := propertyValue.(type) {
			case string:
				source[strings.Replace(k, StringSuffix, "", 1)] = propertyValueTyped
				delete(source, k)
			default:
				return fmt.Errorf("unexpected value type %+v - should be string", propertyValueTyped)
			}
		}

		// Recurse into value maps...
		switch propertyValueTyped := propertyValue.(type) {
		case map[string]any:
			if e := remapDataValues(propertyValueTyped); e != nil {
				overallErr = e
			}
		}
	}

	return overallErr
}
