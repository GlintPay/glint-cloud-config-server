package filetypes

import (
	"fmt"
	"github.com/getsops/sops/v3/decrypt"
	"io"

	"github.com/GlintPay/gccs/backend"
	"github.com/rs/zerolog/log"
	"sigs.k8s.io/yaml"
)

const SopsMetadataKey = "sops" // TODO Rename to distinguish from real, user-level config

type Decrypter interface {
	Decrypt(data []byte) ([]byte, error)
}

type SopsDecrypter struct {
}

func (s SopsDecrypter) Decrypt(data []byte) ([]byte, error) {
	return decrypt.Data(data, "yaml")
}

func FromYamlToMap(f backend.File, decrypter Decrypter) (map[string]any, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	wasSopsDecrypted := false

	if decrypter != nil {
		// Check if it's a valid YAML and if it's encrypted
		hasSops, err := hasSopsMetadata(bytes) // FIXME Double Unmarshal
		if err != nil {
			return nil, err
		}

		// If it has SOPS metadata, decrypt it
		if hasSops {
			decrypted, err := decrypter.Decrypt(bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt SOPS-encrypted content: %w", err)
			}
			bytes = decrypted
			wasSopsDecrypted = true
		}
	}

	// Parse the YAML into a map
	var mapStructuredData map[string]any
	if err := yaml.Unmarshal(bytes, &mapStructuredData); err != nil {
		return nil, err
	}

	if wasSopsDecrypted {
		delete(mapStructuredData, SopsMetadataKey)
	}

	return mapStructuredData, nil
}

// hasSopsMetadata checks if the provided YAML content contains SOPS metadata
func hasSopsMetadata(data []byte) (bool, error) {
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return false, fmt.Errorf("error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type map[string]interface {}")
	}
	_, hasSops := m[SopsMetadataKey]
	return hasSops, nil
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
