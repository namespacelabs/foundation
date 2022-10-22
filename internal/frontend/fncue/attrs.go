// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import (
	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
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
