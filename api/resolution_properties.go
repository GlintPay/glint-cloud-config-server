package api

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"strings"
)

const UnresolvedPropertyResult = ""

type PropertiesResolver struct {
	data     ResolvedConfigValues
	messages []string
}

func (pr *PropertiesResolver) resolvePlaceholdersFromTop() ResolvedConfigValues {
	return pr.resolvePlaceholders(pr.data)
}

func (pr *PropertiesResolver) resolvePlaceholders(currentMap map[string]interface{}) ResolvedConfigValues {
	for propertyName, v := range currentMap {
		switch typedVal := v.(type) {
		case map[string]interface{}:
			pr.resolvePlaceholders(typedVal)
		case []string:
			// TODO Incomplete
			// for propertyName, v := range reconciled {
			//	resolvePlaceholders(typedVal)
			// }
		case string:
			pr.resolveString(currentMap, propertyName, typedVal, "")
		}
	}
	return pr.data
}

// TODO Should missing properties be a configurable fatal error?
func (pr *PropertiesResolver) resolveString(currentMap map[string]interface{}, propertyName string, value string, prefix string) {

	currentMap[propertyName] = placeholderRegex.ReplaceAllStringFunc(value, func(foundMatch string) string {
		sourcePropertyWithDefault := pr.getPropertyClauseFromMatch(foundMatch)
		if sourcePropertyWithDefault[0] == "" {
			// ${} is not acceptable
			pr.addMessage("Missing placeholder [%s] for property [%s]", foundMatch, propertyName)
			return UnresolvedPropertyResult
		}

		if currVal, ok := pr.data[sourcePropertyWithDefault[0]]; ok {
			switch currValStr := currVal.(type) {
			case string:
				if strings.Contains(currValStr, "${") {
					// recurse to resolve placeholder...
					pr.resolveString(currentMap, sourcePropertyWithDefault[0], currValStr, prefix+"  ")
				} else {
					// this value is fine
					return currValStr
				}
			default:
				// this value is fine, but convert to a string
				return fmt.Sprintf("%v", currVal)
			}
		}

		// Re-check post recurse
		if updatedPropertyValue, ok := pr.data[sourcePropertyWithDefault[0]]; ok {
			return updatedPropertyValue.(string)
		}

		// Not found, do we have a default value?
		if len(sourcePropertyWithDefault) < 2 {
			// No match, no default
			pr.addMessage("Missing value for property [%s]", sourcePropertyWithDefault[0])
		} else if len(sourcePropertyWithDefault) > 1 {
			// No match, use available default
			return sourcePropertyWithDefault[1]
		}

		return UnresolvedPropertyResult
	})
}

func (pr *PropertiesResolver) getPropertyClauseFromMatch(match string) []string {
	return strings.Split(strings.TrimSpace(match[2:len(match)-1]), ":")
}

func (pr *PropertiesResolver) addMessage(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	pr.messages = append(pr.messages, msg)
	log.Warn().Msg(msg)
}
