package filetypes

import (
	"fmt"
	"io"

	"github.com/GlintPay/gccs/backend"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

// hasSopsMetadata checks if the provided YAML content contains SOPS metadata
func hasSopsMetadata(data []byte) (bool, error) {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf("error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
	}
	_, hasSops := m["sops"]
	return hasSops, nil
}

func FromYamlToMap(f backend.File) (map[string]any, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check if it's a valid YAML and if it's encrypted
	hasSops, err := hasSopsMetadata(bytes)
	if err != nil {
		return nil, err
	}

	// If it has SOPS metadata, decrypt it
	if hasSops {
		decrypted, err := decrypt.Data(bytes, "yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt SOPS-encrypted content: %w", err)
		}
		bytes = decrypted
	}

	// Parse the YAML into a map
	var mapStructuredData map[string]any
	if err := yaml.Unmarshal(bytes, &mapStructuredData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML content: %w", err)
	}

	return mapStructuredData, nil
}

func ToBytes(f backend.File) ([]byte, error) {
	reader, err := f.Data().Reader()
	if err != nil {
		return nil, err
	}

	defer func(reader io.ReadCloser) {
		if e := reader.Close(); e != nil {
			log.Error().Err(e).Msg("close")
		}
	}(reader)

	return io.ReadAll(reader)
}
