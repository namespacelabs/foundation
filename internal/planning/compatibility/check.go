// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package compatibility

import (
	enverr "namespacelabs.dev/foundation/internal/fnerrors/env"
	"namespacelabs.dev/foundation/schema"
)

func CheckCompatible(env *schema.Environment, srv *schema.Server) error {
	for _, req := range srv.GetEnvironmentRequirement() {
		for _, r := range req.GetEnvironmentHasLabel() {
			if !hasLabel(env, r, true) {
				return enverr.IncompatibleEnvironmentErr{
					Env:               env,
					ServerPackageName: schema.PackageName(srv.PackageName),
					RequirementOwner:  schema.PackageName(req.Package),
					RequiredLabel:     r,
				}
			}
		}

		for _, r := range req.GetEnvironmentDoesNotHaveLabel() {
			if hasLabel(env, r, false) {
				return enverr.IncompatibleEnvironmentErr{
					Env:               env,
					ServerPackageName: schema.PackageName(srv.PackageName),
					RequirementOwner:  schema.PackageName(req.Package),
					IncompatibleLabel: r,
				}
			}
		}
	}

	return nil
}

func hasLabel(env *schema.Environment, lbl *schema.Label, matchEmpty bool) bool {
	for _, x := range env.GetLabels() {
		if x.Name == lbl.Name {
			return x.Value == lbl.Value || (matchEmpty && lbl.Value == "")
		}
	}

	return false
}
