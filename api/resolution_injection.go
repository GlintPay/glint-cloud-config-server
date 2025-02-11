package api

import (
	"strings"
)

type InjectedProperties map[string]any

func preprocess(key string) bool {
	return strings.HasPrefix(key, "^")
}

func postprocess(key string) bool {
	return !strings.HasPrefix(key, "^")
}
