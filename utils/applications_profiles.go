package utils

import (
	"strings"
)

const (
	DefaultApplicationName       = "application"
	DefaultApplicationNamePrefix = DefaultApplicationName + "-"
	BaseLevel                    = DefaultApplicationName + "."
)

func SplitApplicationNames(csv string) []string {
	return splitNonEmpty(csv)
}

func SplitProfileNames(csv string) []string {
	return splitNonEmpty(csv)
}

func splitNonEmpty(csv string) []string {
	array := strings.Split(csv, ",")
	adjusted := make([]string, 0)
	for _, each := range array {
		trimmed := strings.TrimSpace(each)
		if trimmed != "" {
			adjusted = append(adjusted, trimmed)
		}
	}
	return adjusted
}
