package utils

import "strings"

func StripGitPrefix(name string) string {
	arr := strings.SplitAfter(name, "/")
	return arr[len(arr)-1]
}
