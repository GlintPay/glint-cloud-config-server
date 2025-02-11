package sops

import (
	"fmt"

	"github.com/getsops/sops/v3/decrypt"
	"gopkg.in/yaml.v3"
)

// SopsMetadata represents the SOPS metadata structure in encrypted files
type SopsMetadata struct {
	Kms          any `yaml:"kms,omitempty"`
	GcpKms       any `yaml:"gcp_kms,omitempty"`
	AzureKv      any `yaml:"azure_kv,omitempty"`
	LastModified string      `yaml:"lastmodified,omitempty"`
	Mac          string      `yaml:"mac,omitempty"`
	Version      string      `yaml:"version,omitempty"`
}

// IsEncrypted checks if the provided YAML content contains SOPS metadata
func IsEncrypted(data []byte) bool {
	var content map[string]any
	if err := yaml.Unmarshal(data, &content); err != nil {
		return false
	}
	_, hasSops := content["sops"]
	return hasSops
}

// DecryptYAML attempts to decrypt SOPS-encrypted YAML content
// Returns the decrypted content if successful, original content if not encrypted,
// or error if decryption fails
func DecryptYAML(data []byte) ([]byte, error) {
	if !IsEncrypted(data) {
		return data, nil
	}

	decrypted, err := decrypt.Data(data, "yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt SOPS-encrypted content: %w", err)
	}

	return decrypted, nil
}
