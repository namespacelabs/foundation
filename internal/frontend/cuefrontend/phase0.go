// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"io/fs"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func parsePackage(ctx context.Context, evalctx *fncue.EvalCtx, pl workspace.EarlyPackageLoader, loc workspace.Location) (*fncue.Partial, error) {
	return tasks.Return(ctx, tasks.Action("cue.package.parse").LogLevel(1).Scope(loc.PackageName), func(ctx context.Context) (*fncue.Partial, error) {
		if st, err := fs.Stat(fnfs.Local(loc.Module.Abs()), loc.Rel()); err != nil {
			if os.IsNotExist(err) {
				return nil, fnerrors.UserError(nil, "%s: package does not exist", loc.PackageName)
			}
			return nil, err
		} else if !st.IsDir() {
			return nil, fnerrors.UserError(loc, "expected a directory")
		}

		firstPass, err := evalctx.EvalPackage(ctx, loc.PackageName.String())
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing package")
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return nil, err
		}

		inputs := newFuncs().
			WithFetcher(fncue.ProtoloadIKw, FetchProto(pl, fsys, loc)).
			WithFetcher(fncue.ResourceIKw, FetchResource(fsys, loc)).
			WithFetcher(fncue.PackageIKW, FetchPackage(pl))

			// Load packages without the serialization lock held.
		for _, k := range firstPass.Left {
			if k.Key == fncue.PackageIKW {
				if _, err := FetchPackage(pl)(ctx, firstPass.CueV.LookupCuePath(k.Target)); err != nil {
					return nil, err
				}
			}
		}

		return fncue.SerializedEval(firstPass, func() (*fncue.Partial, error) {
			newV, newLeft, err := applyInputs(ctx, inputs, &firstPass.CueV, firstPass.Left)
			if err != nil {
				return nil, fnerrors.Wrapf(loc, err, "evaluating package")
			}

			parsedPartial := *firstPass
			parsedPartial.Set(newV)
			parsedPartial.Left = newLeft

			return &parsedPartial, nil
		})
	})
}
