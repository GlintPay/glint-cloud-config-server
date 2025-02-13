package filetypes

import (
	"fmt"
	"io"

	"github.com/GlintPay/gccs/backend"
	"github.com/getsops/sops/v3/decrypt"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

func FromYamlToMap(f backend.File) (map[string]any, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, err
	}

	// Check if it's a valid YAML and if it's encrypted
	var mapStructuredData map[string]any
	if err := yaml.Unmarshal(bytes, &mapStructuredData); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
	}

	// If it has SOPS metadata, decrypt it
	if _, hasSops := mapStructuredData["sops"]; hasSops {
		decrypted, err := decrypt.Data(bytes, "yaml")
		if err != nil {
			return nil, err
		}
		// Clear the map and unmarshal the decrypted data
		mapStructuredData = make(map[string]any)
		if err := yaml.Unmarshal(decrypted, &mapStructuredData); err != nil {
			return nil, err
		}
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
