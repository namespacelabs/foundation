// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package stringscol

func SliceContains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}

	return false
}

func Without(strs []string, str string) []string {
	var newStrs []string
	for _, s := range strs {
		if s != str {
			newStrs = append(newStrs, s)
		}
	}
	return newStrs
}