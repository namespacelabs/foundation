// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func RequireFeature(module *pkggraph.Module, feature string) error {
	if slices.Contains(module.Workspace.EnabledFeatures, feature) {
		return nil
	}

	return fnerrors.New("feature %q is not enabled by default, as it is still experimental. It can be enabled by adding %q to %q in %q",
		feature, feature, "enabledFeatures", module.DefinitionFile())
}
