package filetypes

import (
	"github.com/GlintPay/gccs/backend"
	"github.com/GlintPay/gccs/sops"
	"github.com/rs/zerolog/log"
	"io"
	"sigs.k8s.io/yaml"
)

func FromYamlToMap(f backend.File) (map[string]any, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, err
	}

	// Try to decrypt if the content is SOPS-encrypted
	if sops.IsEncrypted(bytes) {
		log.Debug().Str("file", f.Name()).Msg("attempting to decrypt SOPS-encrypted content")
		decrypted, err := sops.DecryptYAML(bytes)
		if err != nil {
			log.Warn().Err(err).Str("file", f.Name()).Msg("failed to decrypt SOPS-encrypted content, using original content")
		} else {
			bytes = decrypted
		}
	}

	var mapStructuredData map[string]any
	if e := yaml.Unmarshal(bytes, &mapStructuredData); e != nil {
		return nil, e
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
