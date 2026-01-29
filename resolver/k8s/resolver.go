package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/GlintPay/gccs/config"
	"sigs.k8s.io/yaml"
)

const (
	PrefixK8sSecret      = "k8s/secret:"
	PrefixK8sConfigMap   = "k8s/configmap:"
	PrefixK8sConfigMapCM = "k8s/cm:" // shorthand for configmap
)

type Resolver struct {
	client *Client
	config config.K8sConfig
}

func NewResolver(client *Client, cfg config.K8sConfig) *Resolver {
	return &Resolver{
		client: client,
		config: cfg,
	}
}

// IsK8sPlaceholder checks if the placeholder starts with a k8s prefix (does not require an instance)
func IsK8sPlaceholder(placeholder string) bool {
	return strings.HasPrefix(placeholder, PrefixK8sSecret) ||
		strings.HasPrefix(placeholder, PrefixK8sConfigMap) ||
		strings.HasPrefix(placeholder, PrefixK8sConfigMapCM)
}

// Resolve fetches the value from Kubernetes.
// Placeholder formats:
//   - k8s/secret:namespace/name/key -> explicit namespace
//   - k8s/secret:name/key           -> uses default namespace
//   - k8s/configmap:namespace/name/key
//   - k8s/configmap:name/key
//
// Returns (value, found, error)
func (r *Resolver) Resolve(ctx context.Context, placeholder string) (string, bool, error) {
	var prefix string
	var isSecret bool

	switch {
	case strings.HasPrefix(placeholder, PrefixK8sSecret):
		prefix = PrefixK8sSecret
		isSecret = true
	case strings.HasPrefix(placeholder, PrefixK8sConfigMap):
		prefix = PrefixK8sConfigMap
		isSecret = false
	case strings.HasPrefix(placeholder, PrefixK8sConfigMapCM):
		prefix = PrefixK8sConfigMapCM
		isSecret = false
	default:
		return "", false, fmt.Errorf("unknown k8s placeholder prefix: %s", placeholder)
	}

	path := strings.TrimPrefix(placeholder, prefix)
	namespace, name, key, subKeys, err := r.parsePath(path)
	if err != nil {
		return "", false, err
	}

	var value string
	var found bool

	if isSecret {
		value, found, err = r.client.GetSecretValue(ctx, namespace, name, key)
	} else {
		value, found, err = r.client.GetConfigMapValue(ctx, namespace, name, key)
	}

	if err != nil || !found || len(subKeys) == 0 {
		return value, found, err
	}

	return navigateYAML(value, subKeys)
}

// parsePath extracts namespace, name, key, and optional sub-keys from the path.
// Formats:
//   - "name/key"                          -> uses default namespace, no sub-keys
//   - "name/key/sub1/sub2"               -> uses default namespace, with sub-keys
//   - "namespace/name/key"               -> explicit namespace, no sub-keys
//   - "namespace/name/key/sub1/sub2"     -> explicit namespace, with sub-keys
//
// Sub-keys are used to navigate into YAML-valued entries within a ConfigMap or Secret.
func (r *Resolver) parsePath(path string) (namespace, name, key string, subKeys []string, err error) {
	parts := strings.Split(path, "/")

	if len(parts) < 2 {
		return "", "", "", nil, fmt.Errorf("invalid k8s placeholder path (need at least 2 segments): %s", path)
	}

	// Heuristic: if default namespace is configured and the path has exactly 2 segments,
	// or if no default namespace is configured the first 3 segments are namespace/name/key.
	// With default namespace: 2 segments = name/key, 3+ = could be either.
	// We disambiguate: if default namespace is set, first two are name/key, rest are sub-keys
	// unless there are 3+ segments and no default namespace.

	if r.config.DefaultNamespace != "" && len(parts) >= 2 {
		if len(parts) == 2 {
			return r.config.DefaultNamespace, parts[0], parts[1], nil, nil
		}
		// 3+ segments: namespace/name/key[/subkeys...]
		return parts[0], parts[1], parts[2], subKeysOrNil(parts[3:]), nil
	}

	if r.config.DefaultNamespace == "" && len(parts) < 3 {
		return "", "", "", nil, fmt.Errorf("no default namespace configured and placeholder missing namespace: %s", path)
	}

	// No default namespace, 3+ segments: namespace/name/key[/subkeys...]
	return parts[0], parts[1], parts[2], subKeysOrNil(parts[3:]), nil
}

func subKeysOrNil(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

// navigateYAML parses a YAML string and navigates into it using the given keys.
func navigateYAML(yamlContent string, keys []string) (string, bool, error) {
	var data interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &data); err != nil {
		return "", false, fmt.Errorf("failed to parse YAML content for sub-key navigation: %w", err)
	}

	current := data
	for _, k := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", false, fmt.Errorf("cannot navigate sub-key [%s]: value is not a map", k)
		}
		current, ok = m[k]
		if !ok {
			return "", false, nil
		}
	}

	// Return the value as a string
	switch v := current.(type) {
	case string:
		return v, true, nil
	default:
		return fmt.Sprintf("%v", v), true, nil
	}
}
