package filetypes

import (
	"fmt"
	"io"

	"github.com/GlintPay/gccs/backend"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

type YamlContext struct{}

func FromYamlToMap(f backend.File, _ YamlContext) (map[string]any, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse the YAML into a map
	var mapStructuredData map[string]any
	if err := yaml.Unmarshal(bytes, &mapStructuredData); err != nil {
		return nil, err
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
