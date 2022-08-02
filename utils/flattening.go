package utils

// Flatten take a hierarchy and flatten it using the tokenizer supplied
func Flatten(m map[string]interface{}, tokenizer func([]string) string) map[string]interface{} {
	var r = make(map[string]interface{})
	flattenRecursive(m, []string{}, func(ks []string, v interface{}) {
		r[tokenizer(ks)] = v
	})
	return r
}

func flattenRecursive(m map[string]interface{}, ks []string, cb func([]string, interface{})) {
	for k, v := range m {
		newks := append(ks, k)
		if newm, ok := v.(map[string]interface{}); ok {

			// Method borrowed from https://github.com/wolfeidau/unflatten/blob/master/unflatten.go with this clause added
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
