// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package env

import (
	"fmt"

	"namespacelabs.dev/foundation/schema"
)

type IncompatibleEnvironmentErr struct {
	Env               *schema.Environment
	ServerPackageName schema.PackageName
	RequirementOwner  schema.PackageName
	RequiredLabel     *schema.Label
	IncompatibleLabel *schema.Label
}

func (err IncompatibleEnvironmentErr) Error() string {
	if err.IncompatibleLabel != nil {
		return fmt.Sprintf("environment %q is incompatible with %q (included by %s), it is not compatible with %s=%q",
			err.Env.Name, err.RequirementOwner, err.ServerPackageName, err.IncompatibleLabel.Name, err.IncompatibleLabel.Value)
	}

	var req string
	if err.RequiredLabel.Value == "" {
		req = fmt.Sprintf("%q", err.RequiredLabel.Name)
	} else {
		req = fmt.Sprintf("%s=%q", err.RequiredLabel.Name, err.RequiredLabel.Value)
	}

	return fmt.Sprintf("environment %q is incompatible with %q (included by %s), it requires %s",
		err.Env.Name, err.RequirementOwner, err.ServerPackageName, req)
}
