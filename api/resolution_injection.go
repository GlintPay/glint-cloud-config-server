package api

import (
	"strings"
)

type InjectedProperties map[string]interface{}

func preprocess(key string) bool {
	return strings.HasPrefix(key, "^")
}

func postprocess(key string) bool {
	return !strings.HasPrefix(key, "^")
}
