package filetypes

import (
	"bytes"
	"io"
	"testing"

	"github.com/GlintPay/gccs/backend"
	"github.com/stretchr/testify/assert"
)

type mockFile struct {
	name    string
	content []byte
}

func (m mockFile) Name() string {
	return m.name
}

func (m mockFile) IsReadable() (bool, string) {
	return true, ".yml"
}

func (m mockFile) ToMap() (map[string]interface{}, error) {
	return FromYamlToMap(m)
}

func (m mockFile) FullyQualifiedName() string {
	return m.name
}

func (m mockFile) Location() string {
	return m.name
}

func (m mockFile) Data() backend.Blob {
	return mockBlob{content: m.content}
}

type mockBlob struct {
	content []byte
}

func (m mockBlob) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(m.content)), nil
}

func TestFromYamlToMapWithSops(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
		expectMap   map[string]interface{}
	}{
		{
			name: "regular yaml",
			content: []byte(`
foo: bar
nested:
  value: test
`),
			expectError: false,
			expectMap: map[string]interface{}{
				"foo": "bar",
				"nested": map[string]interface{}{
					"value": "test",
				},
			},
		},
		{
			name: "yaml with sops metadata",
			content: []byte(`
data:
    api_key: ENC[AES256_GCM,data:abc123]
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
			expectError: false, // Should not error even if decryption fails
			expectMap: map[string]interface{}{
				"data": map[string]interface{}{
					"api_key": "ENC[AES256_GCM,data:abc123]",
				},
				"sops": map[string]interface{}{
					"kms": []interface{}{
						map[string]interface{}{
							"arn":        "arn:aws:kms:eu-west-1:123456789012:key/123",
							"created_at": "2024-02-10T12:00:00Z",
							"enc":        "abc123",
						},
					},
					"gcp_kms":      []interface{}{},
					"azure_kv":      []interface{}{},
					"lastmodified": "2024-02-10T12:00:00Z",
					"mac":         "abc123",
					"version":     "3.7.3",
				},
			},
		},
		{
			name: "invalid yaml",
			content: []byte(`
invalid: : yaml
  - broken structure
`),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := mockFile{
				name:    "test.yml",
				content: tt.content,
			}

			result, err := FromYamlToMap(f)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectMap, result)
			}
		})
	}
}
