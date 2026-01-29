package k8s

import (
	"context"
	"testing"

	"github.com/GlintPay/gccs/config"
	"github.com/stretchr/testify/assert"
)

func TestIsK8sPlaceholder(t *testing.T) {
	tests := []struct {
		placeholder string
		expected    bool
	}{
		{"k8s/secret:default/my-secret/key", true},
		{"k8s/secret:my-secret/key", true},
		{"k8s/configmap:default/my-config/key", true},
		{"k8s/configmap:my-config/key", true},
		{"k8s/cm:default/my-config/key", true},
		{"k8s/cm:my-config/key", true},
		{"some.property.name", false},
		{"k8s/unknown:test/key", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.placeholder, func(t *testing.T) {
			result := IsK8sPlaceholder(tt.placeholder)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolver_parsePath(t *testing.T) {
	tests := []struct {
		name             string
		path             string
		defaultNamespace string
		wantNamespace    string
		wantName         string
		wantKey          string
		wantErr          bool
	}{
		{
			name:             "three segments - explicit namespace",
			path:             "backend/hubspot-api/api-key",
			defaultNamespace: "default",
			wantNamespace:    "backend",
			wantName:         "hubspot-api",
			wantKey:          "api-key",
			wantErr:          false,
		},
		{
			name:             "two segments - uses default namespace",
			path:             "hubspot-api/api-key",
			defaultNamespace: "default",
			wantNamespace:    "default",
			wantName:         "hubspot-api",
			wantKey:          "api-key",
			wantErr:          false,
		},
		{
			name:             "two segments - no default namespace configured",
			path:             "hubspot-api/api-key",
			defaultNamespace: "",
			wantErr:          true,
		},
		{
			name:             "one segment - invalid",
			path:             "just-one",
			defaultNamespace: "default",
			wantErr:          true,
		},
		{
			name:             "four segments - invalid",
			path:             "a/b/c/d",
			defaultNamespace: "default",
			wantErr:          true,
		},
		{
			name:             "empty path - invalid",
			path:             "",
			defaultNamespace: "default",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{
				config: config.K8sConfig{
					DefaultNamespace: tt.defaultNamespace,
				},
			}

			ns, name, key, err := resolver.parsePath(tt.path)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantNamespace, ns)
				assert.Equal(t, tt.wantName, name)
				assert.Equal(t, tt.wantKey, key)
			}
		})
	}
}

// MockClient for testing
type mockClient struct {
	secrets    map[string]string // key: "namespace/name/key"
	configMaps map[string]string
	errors     map[string]error
}

func (m *mockClient) GetSecretValue(ctx context.Context, namespace, name, key string) (string, bool, error) {
	fullKey := namespace + "/" + name + "/" + key
	if err, ok := m.errors[fullKey]; ok {
		return "", false, err
	}
	if val, ok := m.secrets[fullKey]; ok {
		return val, true, nil
	}
	return "", false, nil
}

func (m *mockClient) GetConfigMapValue(ctx context.Context, namespace, name, key string) (string, bool, error) {
	fullKey := namespace + "/" + name + "/" + key
	if err, ok := m.errors[fullKey]; ok {
		return "", false, err
	}
	if val, ok := m.configMaps[fullKey]; ok {
		return val, true, nil
	}
	return "", false, nil
}

// testableResolver wraps Resolver with a mockable client interface
type testableResolver struct {
	config config.K8sConfig
	mock   *mockClient
}

func (r *testableResolver) CanResolve(placeholder string) bool {
	return IsK8sPlaceholder(placeholder)
}

func (r *testableResolver) Resolve(ctx context.Context, placeholder string) (string, bool, error) {
	baseResolver := &Resolver{config: r.config}

	var prefix string
	var isSecret bool

	switch {
	case len(placeholder) > len(PrefixK8sSecret) && placeholder[:len(PrefixK8sSecret)] == PrefixK8sSecret:
		prefix = PrefixK8sSecret
		isSecret = true
	case len(placeholder) > len(PrefixK8sConfigMap) && placeholder[:len(PrefixK8sConfigMap)] == PrefixK8sConfigMap:
		prefix = PrefixK8sConfigMap
		isSecret = false
	case len(placeholder) > len(PrefixK8sConfigMapCM) && placeholder[:len(PrefixK8sConfigMapCM)] == PrefixK8sConfigMapCM:
		prefix = PrefixK8sConfigMapCM
		isSecret = false
	default:
		return "", false, nil
	}

	path := placeholder[len(prefix):]
	namespace, name, key, err := baseResolver.parsePath(path)
	if err != nil {
		return "", false, err
	}

	if isSecret {
		return r.mock.GetSecretValue(ctx, namespace, name, key)
	}
	return r.mock.GetConfigMapValue(ctx, namespace, name, key)
}

func TestResolver_Resolve(t *testing.T) {
	ctx := context.Background()

	mock := &mockClient{
		secrets: map[string]string{
			"backend/hubspot-api/api-key":        "secret-value-123",
			"default/postgres-creds/password":    "db-password",
			"production/redis/auth-token":        "redis-token",
		},
		configMaps: map[string]string{
			"backend/logging-config/level":       "DEBUG",
			"default/feature-flags/enabled":      "true",
		},
		errors: map[string]error{},
	}

	tests := []struct {
		name             string
		placeholder      string
		defaultNamespace string
		wantValue        string
		wantFound        bool
		wantErr          bool
	}{
		{
			name:             "secret with explicit namespace",
			placeholder:      "k8s/secret:backend/hubspot-api/api-key",
			defaultNamespace: "default",
			wantValue:        "secret-value-123",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "secret using default namespace",
			placeholder:      "k8s/secret:postgres-creds/password",
			defaultNamespace: "default",
			wantValue:        "db-password",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "configmap with explicit namespace",
			placeholder:      "k8s/configmap:backend/logging-config/level",
			defaultNamespace: "default",
			wantValue:        "DEBUG",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "configmap using default namespace",
			placeholder:      "k8s/configmap:feature-flags/enabled",
			defaultNamespace: "default",
			wantValue:        "true",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "configmap shorthand (k8s/cm) with explicit namespace",
			placeholder:      "k8s/cm:backend/logging-config/level",
			defaultNamespace: "default",
			wantValue:        "DEBUG",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "configmap shorthand (k8s/cm) using default namespace",
			placeholder:      "k8s/cm:feature-flags/enabled",
			defaultNamespace: "default",
			wantValue:        "true",
			wantFound:        true,
			wantErr:          false,
		},
		{
			name:             "secret not found",
			placeholder:      "k8s/secret:backend/nonexistent/key",
			defaultNamespace: "default",
			wantValue:        "",
			wantFound:        false,
			wantErr:          false,
		},
		{
			name:             "missing default namespace",
			placeholder:      "k8s/secret:my-secret/key",
			defaultNamespace: "",
			wantErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &testableResolver{
				config: config.K8sConfig{
					DefaultNamespace: tt.defaultNamespace,
				},
				mock: mock,
			}

			value, found, err := resolver.Resolve(ctx, tt.placeholder)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantFound, found)
				if found {
					assert.Equal(t, tt.wantValue, value)
				}
			}
		})
	}
}
