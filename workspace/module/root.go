// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package module

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func FindRoot(ctx context.Context, dir string) (*workspace.Root, error) {
	return FindRootWithArgs(ctx, dir, workspace.ModuleAtArgs{})
}

func FindRootWithArgs(ctx context.Context, dir string, args workspace.ModuleAtArgs) (*workspace.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	return findWorkspaceRoot(ctx, abs, args)
}

func findWorkspaceRoot(ctx context.Context, dir string, args workspace.ModuleAtArgs) (*workspace.Root, error) {
	path, err := workspace.FindModuleRoot(dir)
	if err != nil {
		return nil, fnerrors.UserError(nil, "workspace: %w", err)
	}

	data, err := workspace.ModuleAt(ctx, path, args)
	if err != nil {
		return nil, err
	}

	r := workspace.NewRoot(data, data)

	if err := devhost.Prepare(ctx, r); err != nil {
		return r, err
	}

	return r, nil
}
