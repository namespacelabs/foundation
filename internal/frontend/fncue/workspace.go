// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

import (
	"context"
	"io/fs"

	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func EvalWorkspace(ctx context.Context, fsys fs.FS, dir string, files []string) (*Partial, error) {
	bldctx := build.NewContext()

	p := bldctx.NewInstance(dir, func(pos token.Pos, path string) *build.Instance {
		if IsStandardImportPath(path) {
			return nil // Builtin.
		}

		berr := bldctx.NewInstance(dir, nil)
		berr.Err = errors.Promote(fnerrors.Newf("imports not allowed"), "")
		return berr
	})

	pkg := CuePackage{
		PackageContents: &PackageContents{
			Snapshot: fsys,
			RelPath:  ".",
		},
		Files: files,
	}

	if err := parseSources(ctx, p, "_", pkg); err != nil {
		return nil, err
	}

	// The user shouldn't be able to reference the injected scope in the workspace file, e.g. $env.
	return finishInstance(nil, cuecontext.New(), p, pkg, nil /* collectedImports */, nil /* scope */)
}
