// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"fmt"

	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var didYouMeanMap = map[string]string{
	"test":     "tests",
	"secret":   "secrets",
	"resource": "resources",
	"require":  "requires",
}

func ValidateNoExtraFields(loc pkggraph.Location, messagePrefix string, v *fncue.CueV, allowedFields []string) error {
	it, err := v.Val.Fields()
	if err != nil {
		return err
	}

	for it.Next() {
		if !slices.Contains(allowedFields, it.Label()) {
			var didYouMean string
			if dym, ok := didYouMeanMap[it.Label()]; ok && slices.Contains(allowedFields, dym) {
				didYouMean = fmt.Sprintf("Did you mean %q? ", dym)
			}
			return fnerrors.NewWithLocation(loc, "%s field %q is not supported. %sPrefix it with \"_\" if it is intentional.", messagePrefix, it.Label(), didYouMean)
		}
	}

	return nil
}
