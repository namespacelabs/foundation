// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package module

import (
	"context"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
)

func FindRoot(ctx context.Context, dir string) (*parsing.Root, error) {
	return FindRootWithArgs(ctx, dir, parsing.ModuleAtArgs{})
}

func FindRootWithArgs(ctx context.Context, dir string, args parsing.ModuleAtArgs) (*parsing.Root, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	return findWorkspaceRoot(ctx, abs, args)
}

func findWorkspaceRoot(ctx context.Context, dir string, args parsing.ModuleAtArgs) (*parsing.Root, error) {
	path, err := parsing.FindModuleRoot(dir)
	if err != nil {
		return nil, fnerrors.New("workspace: %w", err)
	}

	data, err := parsing.ModuleAt(ctx, path, args)
	if err != nil {
		return nil, err
	}

	r := parsing.NewRoot(data, data)

	if err := devhost.Prepare(ctx, r); err != nil {
		return r, err
	}

	return r, nil
}
