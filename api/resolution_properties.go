package api

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/GlintPay/gccs/config"
	"github.com/GlintPay/gccs/resolver/k8s"
	"github.com/Masterminds/sprig"
	"github.com/rs/zerolog/log"
	"regexp"
)

const UnresolvedPropertyResult = ""

type PropertiesResolvable interface {
	resolvePlaceholdersFromTop() (ResolvedConfigValues, error)
}

type PropertiesResolver struct {
	ctx      context.Context
	data     ResolvedConfigValues
	error    error
	messages []string

	templateConfig config.GoTemplate
	templatesData  map[string]any
	k8sResolver    *k8s.Resolver
}

var placeholderRegex = regexp.MustCompile(`\${([^}]*)}`)

func (pr *PropertiesResolver) resolvePlaceholdersFromTop() (ResolvedConfigValues, error) {
	return pr.resolvePlaceholders(pr.data)
}

func (pr *PropertiesResolver) resolvePlaceholders(currentMap map[string]any) (ResolvedConfigValues, error) {
	for propertyName, v := range currentMap {
		switch typedVal := v.(type) {
		case map[string]any:
			_, _ = pr.resolvePlaceholders(typedVal)
		case []any:
			resolved := make([]any, len(typedVal))
			stack := newStack()
			for i, eachUnresolved := range typedVal {
				switch typed := eachUnresolved.(type) {
				case string:
					resolved[i] = pr.resolveString(currentMap, propertyName, typed, stack)
				case map[string]any:
					_, _ = pr.resolvePlaceholders(typed) // ignore results
					resolved[i] = typed
				default:
					stringVal := fmt.Sprintf("%v", eachUnresolved)
					resolved[i] = pr.resolveString(currentMap, propertyName, stringVal, stack)
				}
			}
			currentMap[propertyName] = resolved // replace the whole thing
		case string:
			currentMap[propertyName] = pr.resolveString(currentMap, propertyName, typedVal, newStack())
		}
	}
	return pr.data, pr.error
}

var sprigFuncs = sprig.TxtFuncMap()

var customFuncs = template.FuncMap{
	"dashToUnderscore": func(value string) string {
		return strings.ReplaceAll(value, "-", "_")
	},
}

// TODO Should missing properties be a configurable fatal error?
func (pr *PropertiesResolver) resolveString(currentMap map[string]any, propertyName string, value string, stack map[string]any) string {
	goTemplatesResult := value

	// Look for possible Go templates
	if strings.Contains(value, pr.templateConfig.LeftDelim) && strings.Contains(value, pr.templateConfig.RightDelim) {
		var buf strings.Builder
		tmpl, e := template.New("").Funcs(sprigFuncs).Funcs(customFuncs).Delims(pr.templateConfig.LeftDelim, pr.templateConfig.RightDelim).Parse(value)
		if e != nil {
			pr.error = e
			return ""
		}

		if e := tmpl.Execute(&buf, pr.templatesData); e != nil {
			pr.error = e
			return ""
		}

		goTemplatesResult = buf.String()
	}

	if pr.error != nil {
		return ""
	}

	propertiesResult := placeholderRegex.ReplaceAllStringFunc(goTemplatesResult, func(foundMatch string) string {
		// Extract the content between ${ and }
		placeholderContent := strings.TrimSpace(foundMatch[2 : len(foundMatch)-1])
		if placeholderContent == "" {
			// ${} is not acceptable
			pr.addMessage("Missing placeholder [%s] for property [%s]", foundMatch, propertyName)
			return UnresolvedPropertyResult
		}

		// Check for K8s placeholders first (before splitting on colon)
		if pr.k8sResolver != nil && pr.k8sResolver.CanResolve(placeholderContent) {
			k8sPlaceholder, defaultValue := pr.parseK8sPlaceholderWithDefault(placeholderContent)
			if val, ok, err := pr.k8sResolver.Resolve(pr.ctx, k8sPlaceholder); err != nil {
				pr.error = err
				return UnresolvedPropertyResult
			} else if ok {
				return val
			}
			// Not found - use default if available
			if defaultValue != "" {
				return defaultValue
			}
			pr.addMessage("Missing K8s value for [%s]", k8sPlaceholder)
			return UnresolvedPropertyResult
		}

		// Standard property placeholder handling
		sourcePropertyWithDefault := pr.getPropertyClauseFromMatch(foundMatch)
		if sourcePropertyWithDefault[0] == "" {
			// ${} is not acceptable
			pr.addMessage("Missing placeholder [%s] for property [%s]", foundMatch, propertyName)
			return UnresolvedPropertyResult
		}

		if currVal, ok := pr.resolvePropertyName(sourcePropertyWithDefault[0]); ok {
			switch currValStr := currVal.(type) {
			case string:
				if strings.Contains(currValStr, "${") {
					// recurse to resolve placeholder...
					propName := sourcePropertyWithDefault[0]

					///////////// Handle stack overflows
					if stack != nil && stack[propName] != nil {
						pr.error = fmt.Errorf("stack overflow found when resolving ${%s}", propName)
						return ""
					}
					stack[propName] = true
					/////////////

					currentMap[propName] = pr.resolveString(currentMap, propName, currValStr, stack)
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
		if updatedPropertyValue, ok := pr.resolvePropertyName(sourcePropertyWithDefault[0]); ok {
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

	return propertiesResult
}

func (pr *PropertiesResolver) resolvePropertyName(name string) (any, bool) {
	val, ok := pr.data[name]
	return val, ok
}

func (pr *PropertiesResolver) getPropertyClauseFromMatch(match string) []string {
	return strings.Split(strings.TrimSpace(match[2:len(match)-1]), ":")
}

// parseK8sPlaceholderWithDefault handles K8s placeholders which have the format:
// k8s/secret:namespace/name/key or k8s/secret:namespace/name/key:defaultValue
// The colon after secret/configmap/cm is part of the prefix, but a trailing colon indicates a default.
func (pr *PropertiesResolver) parseK8sPlaceholderWithDefault(placeholder string) (string, string) {
	// Find the prefix (k8s/secret:, k8s/configmap:, or k8s/cm:)
	var prefixEnd int
	if strings.HasPrefix(placeholder, k8s.PrefixK8sSecret) {
		prefixEnd = len(k8s.PrefixK8sSecret)
	} else if strings.HasPrefix(placeholder, k8s.PrefixK8sConfigMap) {
		prefixEnd = len(k8s.PrefixK8sConfigMap)
	} else if strings.HasPrefix(placeholder, k8s.PrefixK8sConfigMapCM) {
		prefixEnd = len(k8s.PrefixK8sConfigMapCM)
	} else {
		return placeholder, ""
	}

	// The rest is path:default (where default is optional)
	rest := placeholder[prefixEnd:]
	if idx := strings.LastIndex(rest, ":"); idx != -1 {
		// Check if this colon separates path from default value
		// Path format is: namespace/name/key or name/key (2-3 segments separated by /)
		pathPart := rest[:idx]
		segments := strings.Count(pathPart, "/")
		if segments >= 1 && segments <= 2 {
			// Valid path, so the colon separates path from default
			return placeholder[:prefixEnd] + pathPart, rest[idx+1:]
		}
	}

	// No default value
	return placeholder, ""
}

func (pr *PropertiesResolver) addMessage(format string, v ...any) {
	msg := fmt.Sprintf(format, v...)
	pr.messages = append(pr.messages, msg)
	log.Warn().Msg(msg)
}

func newStack() map[string]any {
	return map[string]any{}
}
