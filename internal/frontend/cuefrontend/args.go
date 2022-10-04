// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"encoding/json"
	"fmt"
	"sort"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type ArgsListOrMap struct {
	args []string
}

var _ json.Unmarshaler = &ArgsListOrMap{}

func (args *ArgsListOrMap) Parsed() []string {
	if args == nil {
		return nil
	}
	return args.args
}

func (args *ArgsListOrMap) UnmarshalJSON(contents []byte) error {
	var list []string
	if json.Unmarshal(contents, &list) == nil {
		args.args = list
		return nil
	}

	var m map[string]string
	if json.Unmarshal(contents, &m) == nil {
		for k, v := range m {
			if v != "" {
				args.args = append(args.args, fmt.Sprintf("--%s=%s", k, v))
			} else {
				args.args = append(args.args, fmt.Sprintf("--%s", k))
			}
		}
		// Ensure deterministic arg order
		sort.Strings(args.args)
		return nil
	}

	return fnerrors.InternalError("args: expected a list of strings, or a map of string to string")
}
