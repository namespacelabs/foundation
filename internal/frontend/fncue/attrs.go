// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncue

import (
	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
)

func WalkAttrs(parent cue.Value, visit func(v cue.Value, key, value string) error) error {
	var errs []error

	parent.Walk(nil, func(v cue.Value) {
		attrs := v.Attributes(cue.ValueAttr)
		for _, attr := range attrs {
			if attr.Name() != "fn" {
				continue
			}

			for k := 0; k < attr.NumArgs(); k++ {
				key, value := attr.Arg(k)
				if err := visit(v, key, value); err != nil {
					errs = append(errs, err)
				}
			}
		}
	})

	return multierr.New(errs...)
}
