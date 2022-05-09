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
	var v *fncue.Partial
	if err := tasks.Action("cue.package.parse").Scope(loc.PackageName).Run(ctx, func(ctx context.Context) error {
		if st, err := fs.Stat(fnfs.Local(loc.Module.Abs()), loc.Rel()); err != nil {
			if os.IsNotExist(err) {
				return fnerrors.UserError(nil, "%s: package does not exist", loc.PackageName)
			}
			return err
		} else if !st.IsDir() {
			return fnerrors.UserError(loc, "expected a directory")
		}

		firstPass, err := evalctx.Eval(ctx, loc.PackageName.String())
		if err != nil {
			return fnerrors.Wrapf(loc, err, "parsing package")
		}

		fsys, err := pl.WorkspaceOf(ctx, loc.Module)
		if err != nil {
			return err
		}

		inputs := newFuncs().
			WithFetcher(fncue.ServiceIKw, FetchService(pl)).
			WithFetcher(fncue.ProtoloadIKw, FetchProto(fsys, loc)).
			WithFetcher(fncue.ResourceIKw, FetchResource(fsys, loc)).
			WithFetcher(fncue.PackageIKW, FetchPackage(pl))

		newV, newLeft, err := applyInputs(ctx, inputs, &firstPass.CueV, firstPass.Left)
		if err != nil {
			return fnerrors.Wrapf(loc, err, "evaluating package")
		}

		parsedPartial := *firstPass
		parsedPartial.Val = newV.Val
		parsedPartial.Left = newLeft
		v = &parsedPartial

		return nil
	}); err != nil {
		return nil, err
	}

	return v, nil
}
