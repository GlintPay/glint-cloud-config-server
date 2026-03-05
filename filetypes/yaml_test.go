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

func TestFromYamlToMap(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
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
			expectError: false,
			expectMap: map[string]any{
				"foo": "bar",
				"nested": map[string]any{
					"value": "test",
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

			result, err := FromYamlToMap(f, YamlContext{})
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectMap, result)
			}
		})
	}
}
