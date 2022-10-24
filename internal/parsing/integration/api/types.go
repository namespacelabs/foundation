// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Integration[ServerData any, BuildData any] interface {
	ApplyToServer(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, data ServerData) error

	ApplyToTest(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, pkg *pkggraph.Package, test *schema.Test, data ServerData) error

	CreateBinary(ctx context.Context, env *schema.Environment, pl pkggraph.PackageLoader, loc pkggraph.Location, data BuildData) (*schema.Binary, error)
}
