// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

type MinimalPackageLoader interface {
	LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error)
}

type PackageLoader interface {
	Resolve(ctx context.Context, packageName schema.PackageName) (Location, error)
	LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error)
}

type SealedPackageLoader interface {
	PackageLoader

	Modules() []*Module
	Packages() []*Package
}
