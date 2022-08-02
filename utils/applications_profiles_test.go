package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSplitApplicationNames(t *testing.T) {
	tests := []csvExpectation{
		{csv: "production-usa,b,c", want: []string{"production-usa", "b", "c"}},
		{csv: "a, ,c ", want: []string{"a", "c"}},
		{csv: "  ,   ,, ", want: []string{}},
		{csv: "", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.csv, func(t *testing.T) {
			assert.Equal(t, tt.want, SplitApplicationNames(tt.csv))
		})
	}
}

func TestSplitProfileNames(t *testing.T) {
	tests := []csvExpectation{
		{csv: "production-usa,b,c", want: []string{"production-usa", "b", "c"}},
		{csv: "a, ,c ", want: []string{"a", "c"}},
		{csv: "  ,   ,, ", want: []string{}},
		{csv: "", want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.csv, func(t *testing.T) {
			assert.Equal(t, tt.want, SplitProfileNames(tt.csv))
		})
	}
}

type csvExpectation struct {
	csv  string
	want []string
}
