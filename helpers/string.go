package helpers

import (
	"fmt"
	"path/filepath"
)

func AbsPath(path string) string {
	absLocation, err := filepath.Abs(path)
	if err != nil {
		return fmt.Sprintf("<<error: %s>>", err.Error())
	}
	return absLocation
}

// UniqueStrings process the string slice and returns a slice where duplicate strings are
// removed. The order of strings is not touched.  It does not assume a specific order.
func UniqueStrings(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}
