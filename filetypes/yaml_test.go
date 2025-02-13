package filetypes

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/GlintPay/gccs/backend"
	"github.com/stretchr/testify/assert"
)

type mockFile struct {
	name    string
	content []byte
}

type mockDecrypter struct{}

func (m mockDecrypter) Decrypt(data []byte) ([]byte, error) {
	return data, nil
}

type erroringDecrypter struct{}

func (m erroringDecrypter) Decrypt(data []byte) ([]byte, error) {
	return nil, errors.New("error")
}

func (m mockFile) Name() string {
	return m.name
}

func (m mockFile) IsReadable() (bool, string) {
	panic("unexpected")
}

func (m mockFile) ToMap() (map[string]any, error) {
	panic("unexpected")
}

func (m mockFile) FullyQualifiedName() string {
	panic("unexpected")
}

func (m mockFile) Location() string {
	panic("unexpected")
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
		decrypter   Decrypter
		expectError bool
		expectMap   map[string]any
	}{
		{
			name: "regular yaml",
			content: []byte(`
foo: bar
nested:
  value: test
`),

			decrypter:   mockDecrypter{},
			expectError: false,
			expectMap: map[string]any{
				"foo": "bar",
				"nested": map[string]any{
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
			decrypter:   mockDecrypter{},
			expectError: false,
			expectMap: map[string]any{
				"data": map[string]any{
					"api_key": "ENC[AES256_GCM,data:abc123]",
				},
			},
		},
		{
			name: "decryption fails",
			content: []byte(`sops:
  kms: []
  gcp_kms: []
  azure_kv: []
  lastmodified: "2024-02-10T12:00:00Z"
  mac: "abc123"
  version: "3.7.3"
`),
			decrypter:   erroringDecrypter{},
			expectError: true,
			expectMap:   nil,
		},
		{
			name: "invalid yaml",
			content: []byte(`
invalid: : yaml
  - broken structure
`),
			decrypter:   mockDecrypter{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := mockFile{
				name:    "test.yml",
				content: tt.content,
			}

			result, err := FromYamlToMap(f, tt.decrypter)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectMap, result)
			}
		})
	}
}
