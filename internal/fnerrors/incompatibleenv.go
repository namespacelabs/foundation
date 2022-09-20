// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnerrors

import (
	"fmt"

	"namespacelabs.dev/foundation/schema"
)

type IncompatibleEnvironmentErr struct {
	Env               *schema.Environment
	Server            *schema.Server
	RequirementOwner  schema.PackageName
	RequiredLabel     *schema.Label
	IncompatibleLabel *schema.Label
}

func (err IncompatibleEnvironmentErr) Error() string {
	if err.IncompatibleLabel != nil {
		return fmt.Sprintf("environment %q is incompatible with %q (included by %s), it is not compatible with %s=%q",
			err.Env.Name, err.RequirementOwner, err.Server.PackageName, err.IncompatibleLabel.Name, err.IncompatibleLabel.Value)
	}

	return fmt.Sprintf("environment %q is incompatible with %q (included by %s), it requires %s=%q",
		err.Env.Name, err.RequirementOwner, err.Server.PackageName, err.RequiredLabel.Name, err.RequiredLabel.Value)
}
