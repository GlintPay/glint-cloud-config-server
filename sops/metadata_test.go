package sops

import (
	"testing"
)

func TestHasSopsMetadata(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    bool
	}{
		{
			name:    "empty content",
			content: []byte{},
			want:    false,
		},
		{
			name:    "invalid yaml",
			content: []byte("invalid yaml content"),
			want:    false,
		},
		{
			name:    "valid yaml without sops",
			content: []byte("key: value"),
			want:    false,
		},
		{
			name: "valid yaml with sops",
			content: []byte(`
sops:
  kms: []
  gcp_kms: []
  azure_kv: []
  lastmodified: '2023-01-01T00:00:00Z'
  mac: DEADBEEF
  version: 1.2.3
key: value
`),
			want: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := HasSopsMetadata(tt.content)

			if result != tt.want {
				t.Errorf("HasSopsMetadata() = %v, want %v", result, tt.want)
			}
		})
	}
}
