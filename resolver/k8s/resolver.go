package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/GlintPay/gccs/config"
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

// CanResolve checks if the placeholder starts with a k8s prefix
func (r *Resolver) CanResolve(placeholder string) bool {
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
	namespace, name, key, err := r.parsePath(path)
	if err != nil {
		return "", false, err
	}

	if isSecret {
		return r.client.GetSecretValue(ctx, namespace, name, key)
	}
	return r.client.GetConfigMapValue(ctx, namespace, name, key)
}

// parsePath extracts namespace, name, and key from the path.
// Format: "namespace/name/key" (3 segments) or "name/key" (2 segments, uses default namespace)
func (r *Resolver) parsePath(path string) (namespace, name, key string, err error) {
	parts := strings.Split(path, "/")

	switch len(parts) {
	case 2:
		// name/key - use default namespace
		if r.config.DefaultNamespace == "" {
			return "", "", "", fmt.Errorf("no default namespace configured and placeholder missing namespace: %s", path)
		}
		return r.config.DefaultNamespace, parts[0], parts[1], nil
	case 3:
		// namespace/name/key
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid k8s placeholder path (expected 2 or 3 segments): %s", path)
	}
}
