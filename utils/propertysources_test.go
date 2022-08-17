package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestStripGitPrefix(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		expected string
	}{
		{
			name:     "prod",
			val:      "git@github.com:Org/config.git/application-prod-us.yml",
			expected: "application-prod-us.yml",
		},
		{
			name:     "unexpected",
			val:      "application.yml",
			expected: "application.yml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripGitPrefix(tt.val))
		})
	}
}
