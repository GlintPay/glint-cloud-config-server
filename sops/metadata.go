package sops

import (
	"gopkg.in/yaml.v3"
)

// HasSopsMetadata checks if the provided YAML content contains SOPS metadata
func HasSopsMetadata(data []byte) bool {
	var m map[string]interface{}
	if err := yaml.Unmarshal(data, &m); err != nil {
		return false
	}
	_, hasSops := m["sops"]
	return hasSops
}
