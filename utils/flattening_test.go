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
			args: map[string]any{
				"xxx": map[string]any{
					"currencies": []string{"DEF", "GHI", "JKL"},
					"metadata":   map[string]any{},
				},
				"val":        "yyy",
				"currencies": []string{"USD", "EUR", "ABC"},
				"site":       map[string]any{"retries": 0},
				"timeout":    50,
			},
			expected: map[string]any{
				"xxx.currencies": []string{"DEF", "GHI", "JKL"},
				"xxx.metadata":   map[string]any{},
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
	args      map[string]any
	tokenizer func([]string) string
	expected  map[string]any
}
