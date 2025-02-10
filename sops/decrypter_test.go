package sops

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name: "unencrypted yaml",
			content: []byte(`
foo: bar
baz: qux
`),
			expected: false,
		},
		{
			name: "encrypted yaml with sops metadata",
			content: []byte(`
foo: ENC[AES256_GCM,data:abc123]
sops:
    kms:
        - arn: arn:aws:kms:eu-west-1:123456789012:key/123
          created_at: "2024-02-10T12:00:00Z"
          enc: abc123
    gcp_kms: []
    azure_kv: []
    lastmodified: "2024-02-10T12:00:00Z"
    mac: abc123
    version: 3.7.3
`),
			expected: true,
		},
		{
			name: "invalid yaml",
			content: []byte(`
invalid: yaml: content
  - not properly indented
    : missing value
`),
			expected: false,
		},
		{
			name:     "empty content",
			content:  []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsEncrypted(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDecryptYAML(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{
			name: "unencrypted yaml",
			content: []byte(`
foo: bar
baz: qux
`),
			expectError: false,
		},
		{
			name: "encrypted yaml without kms access",
			content: []byte(`
foo: ENC[AES256_GCM,data:abc123]
sops:
    kms:
        - arn: arn:aws:kms:eu-west-1:123456789012:key/123
          created_at: "2024-02-10T12:00:00Z"
          enc: abc123
    gcp_kms: []
    azure_kv: []
    lastmodified: "2024-02-10T12:00:00Z"
    mac: abc123
    version: 3.7.3
`),
			expectError: true, // Will fail because we don't have KMS access in tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := DecryptYAML(tt.content)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.content, result) // For unencrypted content, should return as-is
			}
		})
	}
}
