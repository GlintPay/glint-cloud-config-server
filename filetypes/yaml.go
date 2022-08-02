package filetypes

import (
	"github.com/GlintPay/gccs/backend"
	"github.com/rs/zerolog/log"
	"io"
	"io/ioutil"
	"sigs.k8s.io/yaml"
)

func FromYamlToMap(f backend.File) (map[string]interface{}, error) {
	bytes, err := ToBytes(f)
	if err != nil {
		return nil, err
	}

	var mapStructuredData map[string]interface{}
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

	return ioutil.ReadAll(reader)
}
