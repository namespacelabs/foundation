// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/schema"
)

type PackageLoader interface {
	Resolve(ctx context.Context, packageName schema.PackageName) (Location, error)
	LoadByName(ctx context.Context, packageName schema.PackageName) (*Package, error)
}

type ModuleSources struct {
	Module   *Module
	Snapshot fs.FS
}

type SealedPackageLoader interface {
	PackageLoader
	Sources() []ModuleSources
}
