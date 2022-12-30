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

	// Ensure that a package is being loaded.
	// Guaranteed to return immediately if the package is already being loaded concurrently.
	// This is important to avoid deadlocks on cyclic dependencies.
	Ensure(ctx context.Context, packageName schema.PackageName) error
}

type Modules interface {
	Modules() []*Module
}

type SealedPackageLoader interface {
	PackageLoader
	Modules

	Packages() []*Package
}
