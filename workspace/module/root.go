// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package module

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func FindRoot(ctx context.Context, dir string) (*workspace.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	return findWorkspaceRoot(ctx, abs)
}

func findWorkspaceRoot(ctx context.Context, dir string) (*workspace.Root, error) {
	path, err := findroot.Find(dir, findroot.LookForFile(workspace.WorkspaceFilename))
	if err != nil {
		return nil, fnerrors.UserError(nil, "workspace: %w", err)
	}

	w, err := workspace.ModuleAt(path)
	if err != nil {
		return nil, err
	}

	r := workspace.NewRoot(path, w)

	if err := devhost.Prepare(ctx, r); err != nil {
		return r, err
	}

	return r, nil
}