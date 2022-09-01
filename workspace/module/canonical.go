// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package module

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace"
)

func PackageAt(ctx context.Context, dir string) (*workspace.Root, fnfs.Location, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	root, err := findWorkspaceRoot(ctx, abs, workspace.ModuleAtArgs{})
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	rel, err := filepath.Rel(root.Abs(), abs)
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	return root, root.RelPackage(rel), nil
}
