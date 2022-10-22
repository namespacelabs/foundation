// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package module

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
)

func PackageAt(ctx context.Context, dir string) (*parsing.Root, fnfs.Location, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	root, err := findWorkspaceRoot(ctx, abs, parsing.ModuleAtArgs{})
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	rel, err := filepath.Rel(root.Abs(), abs)
	if err != nil {
		return nil, fnfs.Location{}, err
	}

	return root, root.RelPackage(rel), nil
}
