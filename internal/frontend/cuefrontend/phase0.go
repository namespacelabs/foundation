// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"io/fs"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func parsePackage(ctx context.Context, evalctx *fncue.EvalCtx, pl parsing.EarlyPackageLoader, loc pkggraph.Location) (*fncue.Partial, error) {
	return tasks.Return(ctx, tasks.Action("cue.package.parse").LogLevel(1).Scope(loc.PackageName), func(ctx context.Context) (*fncue.Partial, error) {
		if st, err := fs.Stat(fnfs.Local(loc.Module.Abs()), loc.Rel()); err != nil {
			if os.IsNotExist(err) {
				return nil, fnerrors.New("%s: package does not exist", loc.PackageName)
			}
			return nil, err
		} else if !st.IsDir() {
			return nil, fnerrors.NewWithLocation(loc, "expected a directory")
		}

		firstPass, err := evalctx.EvalPackage(ctx, loc.PackageName.String())
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing package failed: %w", err)
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return nil, err
		}

		inputs := newFuncs().
			WithFetcher(fncue.ProtoloadIKw, FetchProto(pl, fsys, loc)).
			WithFetcher(fncue.ResourceIKw, FetchResource(fsys, loc)).
			WithFetcher(fncue.PackageIKW, FetchPackage(pl)).
			WithFetcher(fncue.PackageRefIKW, FetchPackageRef(pl))

		// Load packages without the serialization lock held.
		for _, k := range firstPass.Left {
			if k.Key == fncue.PackageIKW {
				if _, err := FetchPackage(pl)(ctx, firstPass.CueV.Val.LookupPath(k.Target)); err != nil {
					return nil, err
				}
			}
		}

		return fncue.SerializedEval(firstPass, func() (*fncue.Partial, error) {
			newV, newLeft, err := applyInputs(ctx, inputs, &firstPass.CueV, firstPass.Left)
			if err != nil {
				return nil, fnerrors.NewWithLocation(loc, "evaluating package failed: %w", err)
			}

			parsedPartial := *firstPass
			parsedPartial.Val = newV.Val
			parsedPartial.Left = newLeft

			return &parsedPartial, nil
		})
	})
}
