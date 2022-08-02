package filetypes

import (
	"fmt"
	"github.com/GlintPay/gccs/backend"
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
			fmt.Println(e)
		}
	}(reader)

	return ioutil.ReadAll(reader)
}
