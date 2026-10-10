package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Characterization tests pinning existing resolution behaviour.
func Test_resolvePlaceholders_characterization(t *testing.T) {
	tests := []placeholdersTest{
		{
			name: "direct-cycle",
			inputs: map[string]any{
				"a": "${b}",
				"b": "${a}",
			},
			expectation: map[string]any{
				"a": "",
				"b": "",
			},
			expectedErrorMsg: "stack overflow found when resolving",
		},
		{
			name: "self-reference",
			inputs: map[string]any{
				"a": "x${a}",
			},
			expectation: map[string]any{
				"a": "",
			},
			expectedErrorMsg: "stack overflow found when resolving ${a}",
		},
		{
			name: "default-containing-spaces-and-trimmed-placeholder",
			inputs: map[string]any{
				"a": "${ missing:some default }!",
			},
			expectation: map[string]any{
				"a": "some default!",
			},
		},
		{
			name: "non-string-values-are-stringified",
			inputs: map[string]any{
				"n": 42,
				"b": true,
				"a": "${n}-${b}",
			},
			expectation: map[string]any{
				"n": float64(42), // via the JSON deep copy
				"b": true,
				"a": "42-true",
			},
		},
		{
			name: "lists-and-nested-maps",
			inputs: map[string]any{
				"x": "ex",
				"l": []any{"${x}", "plain", map[string]any{"k": "${x}"}, 7},
				"m": map[string]any{"inner": "${x}!"},
			},
			expectation: map[string]any{
				"x": "ex",
				"l": []any{"ex", "plain", map[string]any{"k": "ex"}, "7"},
				"m": map[string]any{"inner": "ex!"},
			},
		},
		{
			name: "templates-run-before-placeholders",
			inputs: map[string]any{
				"x": "value",
				// Template output is itself a placeholder, resolved in the second pass
				"a": `{{ "$" }}{{ "{x}" }}`,
				// ...whereas a placeholder inside a template is just literal text
				"b": `{{ upper "${x}" }}`,
			},
			expectation: map[string]any{
				"x": "value",
				"a": "value",
				"b": "",
			},
			messages: []string{"Missing value for property [X]"},
		},
		{
			name: "template-with-no-delims-left-alone",
			inputs: map[string]any{
				"a": "{{ unterminated",
			},
			expectation: map[string]any{
				"a": "{{ unterminated",
			},
		},
		{
			name: "k8s-placeholder-without-resolver",
			inputs: map[string]any{
				"a": "${k8s/secret:ns/name/key}",
			},
			expectation: map[string]any{
				"a": "",
			},
			expectedErrorMsg: "K8s resolver is not available",
		},
	}

	for _, tt := range tests {
		for i := 1; i <= 20; i++ { // map iteration order is random, so repeat
			t.Run(tt.name, func(t *testing.T) {
				newData := map[string]any{}
				assert.NoError(t, deepCopyViaJSON(tt.inputs, newData))

				rr := PropertiesResolver{
					data:           newData,
					templateConfig: tt.templateConfig.Validate(),
					templatesData:  tt.templatesData,
				}

				result, e := rr.resolvePlaceholdersFromTop()

				if tt.expectedErrorMsg != "" {
					assert.ErrorContains(t, e, tt.expectedErrorMsg)
				} else {
					assert.NoError(t, e)
				}

				if tt.expectedErrorMsg == "" {
					assert.Equal(t, tt.expectation, result)
				}
				assert.ElementsMatch(t, tt.messages, rr.messages)
			})
		}
	}
}
