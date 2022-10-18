// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compatibility

import (
	enverr "namespacelabs.dev/foundation/internal/fnerrors/env"
	"namespacelabs.dev/foundation/schema"
)

func CheckCompatible(env *schema.Environment, srv *schema.Server) error {
	for _, req := range srv.GetEnvironmentRequirement() {
		for _, r := range req.GetEnvironmentHasLabel() {
			if !env.HasLabel(r) {
				return enverr.IncompatibleEnvironmentErr{
					Env:               env,
					ServerPackageName: schema.PackageName(srv.PackageName),
					RequirementOwner:  schema.PackageName(req.Package),
					RequiredLabel:     r,
				}
			}
		}

		for _, r := range req.GetEnvironmentDoesNotHaveLabel() {
			if env.HasLabel(r) {
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
