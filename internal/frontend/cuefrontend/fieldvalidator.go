// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"fmt"
	"math"

	"github.com/agext/levenshtein"
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ValidateNoExtraFields(loc pkggraph.Location, messagePrefix string, v *fncue.CueV, allowedFields []string) error {
	it, err := v.Val.Fields()
	if err != nil {
		return err
	}

	for it.Next() {
		fieldName := it.Label()
		if !slices.Contains(allowedFields, fieldName) {
			return fnerrors.NewWithLocation(loc, "%s field %q is not supported. %sPrefix it with \"_\" if it is intentional.",
				messagePrefix, fieldName, didYouMean(fieldName, allowedFields))
		}
	}

	return nil
}

func didYouMean(given string, allowed []string) string {
	suggestion := ""
	lowest := math.MaxInt
	for _, word := range allowed {
		dist := levenshtein.Distance(given, word, nil)
		if dist < 3 && dist < lowest {
			suggestion = word
			lowest = dist
		}
	}

	if suggestion != "" {
		return fmt.Sprintf("\n\n  Did you mean %q?\n\n", suggestion)
	}

	return suggestion
}
