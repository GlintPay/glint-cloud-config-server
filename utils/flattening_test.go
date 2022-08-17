package utils

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestFlatten(t *testing.T) {
	tests := []tcase{
		{
			name: "",
			args: map[string]interface{}{
				"xxx": map[string]interface{}{
					"currencies": []string{"DEF", "GHI", "JKL"},
					"metadata":   map[string]interface{}{},
				},
				"val":        "yyy",
				"currencies": []string{"USD", "EUR", "ABC"},
				"site":       map[string]interface{}{"retries": 0},
				"timeout":    50,
			},
			expected: map[string]interface{}{
				"xxx.currencies": []string{"DEF", "GHI", "JKL"},
				"xxx.metadata":   map[string]interface{}{},
				"currencies":     []string{"USD", "EUR", "ABC"},
				"site.retries":   0, "timeout": 50,
				"val": "yyy",
			},
			tokenizer: dotJoiner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Flatten(tt.args, tt.tokenizer))
		})
	}
}

var dotJoiner = func(k []string) string {
	return strings.Join(k, ".")
}

type tcase struct {
	name      string
	args      map[string]interface{}
	tokenizer func([]string) string
	expected  map[string]interface{}
}
