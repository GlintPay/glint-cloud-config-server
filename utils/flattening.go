package utils

// Flatten take a hierarchy and flatten it using the tokenizer supplied
func Flatten(m map[string]any, tokenizer func([]string) string) map[string]any {
	var r = make(map[string]any)
	flattenRecursive(m, []string{}, func(ks []string, v any) {
		r[tokenizer(ks)] = v
	})
	return r
}

func flattenRecursive(m map[string]any, ks []string, cb func([]string, any)) {
	for k, v := range m {
		newks := append(ks, k) //nolint:gocritic
		if newm, ok := v.(map[string]any); ok {

			// Method borrowed from https://github.com/wolfeidau/unflatten/blob/master/flatten.go with this clause added
			// to handle empty map values
			if len(newm) == 0 {
				cb(newks, v)
				continue
			}

			flattenRecursive(newm, newks, cb)
		} else {
			cb(newks, v)
		}
	}
}
