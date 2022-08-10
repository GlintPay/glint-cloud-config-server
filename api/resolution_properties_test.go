package api

import (
	"encoding/json"
	"errors"
	"github.com/GlintPay/gccs/config"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_resolvePlaceholders(t *testing.T) {
	tests := []placeholdersTest{
		{
			name: "multi-level-recurse",
			inputs: map[string]interface{}{
				"b.d.a": "  ${c.d.a}${}",
				"b.a.a": "${c.d.a}",
				"a.a.a": "apple",
				"c.d.a": "${a.d.a} ${animal:fox}",
				"a.d.a": "${a.b.a}",
				"a.a.b": "bye",
				"a.c.c": "${greeting:Hey} ${a.a.c} .",
				"a.d.b": "${a.b.d}",
				"a.b.d": "${a.b.a}",
				"a.e.b": "${missing}  ${a.a.b}",
				"a.d.c": "${a.c.c}",
				"a.a.c": "cat",
				"b.d.b": "${a.a.b}",
				"a.b.a": "${a.a.a}  ",
				"a.b.c": "${a.d.c}",
				"c.e.a": "!  ${c.d.a}  ",
			},
			expectation: map[string]interface{}{
				"a.a.a": "apple",
				"a.a.b": "bye",
				"a.a.c": "cat",
				"a.b.a": "apple  ",
				"a.b.c": "Hey cat .",
				"a.b.d": "apple  ",
				"a.c.c": "Hey cat .",
				"a.d.a": "apple  ",
				"a.d.b": "apple  ",
				"a.d.c": "Hey cat .",
				"a.e.b": "  bye",
				"b.a.a": "apple   fox",
				"b.d.a": "  apple   fox",
				"b.d.b": "bye",
				"c.d.a": "apple   fox",
				"c.e.a": "!  apple   fox  ",
			},
			messages: []string{
				"Missing value for property [missing]",
				"Missing placeholder [${}] for property [b.d.a]",
			},
		},
		{
			name: "templates-good",
			inputs: map[string]interface{}{
				"a": "Application: {{ first .Applications }}, Profile: {{ first .Profiles }}, Underscored: {{ dashToUnderscore (first .Profiles) }} ",
			},
			templatesData: map[string]interface{}{
				"Applications": []string{"accounts", "application"},
				"Profiles":     []string{"prod-uk", "prod", "base"},
			},
			expectation: map[string]interface{}{
				"a": "Application: accounts, Profile: prod-uk, Underscored: prod_uk",
			},
		},
		{
			name: "templates-good-custom-delims",
			inputs: map[string]interface{}{
				"a": "Application: <<< first .Applications >>>, Profile: <<< first .Profiles >>>",
			},
			templateConfig: config.GoTemplate{LeftDelim: "<<<", RightDelim: ">>>"},
			templatesData: map[string]interface{}{
				"Applications": []string{"accounts", "application"},
				"Profiles":     []string{"prod-uk", "prod", "base"},
			},
			expectation: map[string]interface{}{
				"a": "Application: accounts, Profile: prod-uk",
			},
		},
		{
			name: "templates-malformed-1",
			inputs: map[string]interface{}{
				"a": "Application: {{ first .Applications }}, Profile: {{{ first .Profiles }}",
			},
			templatesData: map[string]interface{}{
				"Applications": []string{"accounts", "application"},
				"Profiles":     []string{"prod-uk", "prod", "base"},
			},
			expectation: map[string]interface{}{
				"a": "",
			},
			expectedErrorMsg: "unexpected \"{\" in command",
		},
		{
			name: "templates-bad-data",
			inputs: map[string]interface{}{
				"a": "Application: {{ first .Applications }}, Profile: {{ first .Profiles }}",
			},
			expectation: map[string]interface{}{
				"a": "",
			},
			expectedErrorMsg: "at <first .Applications>: error calling first: runtime error",
		},
		{
			name: "templates-bad",
			inputs: map[string]interface{}{
				"a": "Application: {{ first .Applications }}, Profile: {{ xxxx .Profiles }}",
			},
			templatesData: map[string]interface{}{
				"Applications": []string{"accounts", "application"},
				"Profiles":     []string{"prod-uk", "prod", "base"},
			},
			expectation: map[string]interface{}{
				"a": "",
			},
			expectedErrorMsg: "function \"xxxx\" not defined",
		},
		{
			name: "maps",
			inputs: map[string]interface{}{
				"vals.w": "w",
				"vals.x": "${a}",
				"vals.y": "${b}",
				"vals.z": "${c:3}",
				"vals.a": "${d}",
				"vals.b": "${}",
				"vals.c": "c",
				"a":      1.0,
				"b":      2.0,
			},
			expectation: map[string]interface{}{
				"a":      1.0,
				"b":      2.0,
				"vals.w": "w",
				"vals.x": "1",
				"vals.y": "2",
				"vals.z": "3",
				"vals.a": "",
				"vals.b": "",
				"vals.c": "c",
			},
			messages: []string{
				"Missing value for property [d]",
				"Missing placeholder [${}] for property [vals.b]",
			},
		},
		{
			name: "maps-hier",
			inputs: map[string]interface{}{
				"vals": map[string]interface{}{
					"w": "w",
					"x": "${a}",
					"y": "${b}",
					"z": "${c:3}",
					"a": "${d}",
					"b": "${}",
					"c": "c",
				},
				"a": 1.0,
				"b": 2.0,
			},
			expectation: map[string]interface{}{
				"vals": map[string]interface{}{
					"w": "w",
					"x": "1",
					"y": "2",
					"z": "3",
					"a": "",
					"b": "",
					"c": "c",
				},
				"a": 1.0,
				"b": 2.0,
			},
			messages: []string{
				"Missing value for property [d]",
				"Missing placeholder [${}] for property [b]",
			},
		},
		{
			name: "lists-flat",
			inputs: map[string]interface{}{
				"vals.sub[0]": "w",
				"vals.sub[1]": "${a}",
				"vals.sub[2]": "${b}",
				"vals.sub[3]": "${c:3}",
				"vals.sub[4]": "${d}",
				"vals.sub[5]": "${}",
				"vals.sub[6]": "c",
				"a":           1.0,
				"b":           2.0,
			},
			expectation: map[string]interface{}{
				"a":           1.0,
				"b":           2.0,
				"vals.sub[0]": "w",
				"vals.sub[1]": "1",
				"vals.sub[2]": "2",
				"vals.sub[3]": "3",
				"vals.sub[4]": "",
				"vals.sub[5]": "",
				"vals.sub[6]": "c",
			},
			messages: []string{
				"Missing value for property [d]",
				"Missing placeholder [${}] for property [vals.sub[5]]",
			},
		},
		{
			name: "lists-hier",
			inputs: map[string]interface{}{
				"a": 1.0,
				"b": 2.0,
				"vals": []string{
					"w",
					"${a}",
					"${b}",
					"${c:3}",
					"${d}",
					"${}",
					"c",
				},
			},
			expectation: map[string]interface{}{
				"a": 1.0,
				"b": 2.0,
				"vals": []string{
					"w",
					"1",
					"2",
					"3",
					"",
					"",
					"c",
				},
			},
			messages: []string{
				"Missing value for property [d]",
				"Missing placeholder [${}] for property [vals]",
			},
		},
		{
			name: "overflow",
			inputs: map[string]interface{}{
				"a": "${b}",
				"b": "${a}",
			},
			expectation: map[string]interface{}{
				"a": "",
				"b": "",
			},
			expectedErrorMsg: "stack overflow found when resolving ${",
		},
	}
	for _, tt := range tests {
		for i := 1; i <= 5; i++ {
			t.Run(tt.name, func(t *testing.T) {
				// Resolution is destructive, so let's make a *deep* copy
				newData := map[string]interface{}{}
				e := deepCopyViaJSON(tt.inputs, newData)
				assert.NoError(t, e)

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

				assert.Equal(t, tt.expectation, result)
				assert.ElementsMatch(t, tt.messages, rr.messages)
			})
		}
	}
}

type placeholdersTest struct {
	name             string
	inputs           ResolvedConfigValues
	expectation      ResolvedConfigValues
	expectedErrorMsg string
	messages         []string

	templateConfig config.GoTemplate
	templatesData  map[string]interface{}
}

// Must deal with floats rather than ints if we're going to use this approach
func deepCopyViaJSON(src map[string]interface{}, dest map[string]interface{}) error {
	if src == nil {
		return errors.New("src is nil. You cannot read from a nil map")
	}
	if dest == nil {
		return errors.New("dest is nil. You cannot insert to a nil map")
	}
	jsonStr, err := json.Marshal(src)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonStr, &dest)
	if err != nil {
		return err
	}
	return nil
}
