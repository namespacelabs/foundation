// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type SecretSource interface {
	Load(context.Context, pkggraph.Modules, *schema.PackageRef, *SecretRequest_ServerRef) (*schema.FileContents, error)
	MissingError(*schema.PackageRef, *schema.SecretSpec, schema.PackageName) error
}

type SecretRequest_ServerRef struct {
	PackageName schema.PackageName
	ModuleName  string
	RelPath     string // Relative path within the module.
}
