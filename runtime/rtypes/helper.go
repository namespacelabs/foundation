// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rtypes

import (
	"fmt"
	"sort"
	"strings"

	"cuelang.org/go/cue"
)

func CollectArgs(v cue.Value) ([]*Arg, error) {
	if !v.Exists() {
		return nil, nil
	}

	var args []*Arg

	it, err := v.Fields()
	if err != nil {
		return nil, err
	}

	for it.Next() {
		value, err := it.Value().String()
		if err != nil {
			return nil, err
		}
		args = append(args, &Arg{Name: it.Selector().String(), Value: value})
	}

	return args, nil
}

func FlattenArgs(args []*Arg) []string {
	var result []string
	for _, a := range args {
		result = append(result, fmt.Sprintf("--%s=%s", a.Name, a.Value))
	}
	// Ensure that the list of args is stable.
	sort.Strings(result)
	return result
}

func SplitImage(image string) (string, string) {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}