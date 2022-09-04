// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type Frontend struct {
	loader workspace.EarlyPackageLoader
}

func NewFrontend(pl workspace.EarlyPackageLoader) *Frontend {
	return &Frontend{loader: pl}
}

func (ft Frontend) ParsePackage(ctx context.Context, partial *fncue.Partial, loc workspace.Location, opts workspace.LoadPackageOpts) (*workspace.Package, error) {
	v := &partial.CueV

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	phase1plan := &phase1plan{}
	parsedPkg := &workspace.Package{
		Location: loc,
		Parsed:   phase1plan,
	}

	server := v.LookupPath("server")
	if !server.Exists() {
		return nil, fnerrors.UserError(loc, "Missing server field")
	}

	var parsedSecrets []*schema.SecretSpec
	if secrets := v.LookupPath("secrets"); secrets.Exists() {
		it, err := secrets.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			parsedSecret, err := parseSecret(ctx, loc, it.Label(), it.Value())
			if err != nil {
				return nil, err
			}

			parsedSecrets = append(parsedSecrets, parsedSecret)
		}
	}

	var parsedVolumes []*schema.Volume
	if volumes := v.LookupPath("volumes"); volumes.Exists() {
		var err error
		parsedVolumes, err = cuefrontend.ParseVolumes(ctx, ft.loader, loc, volumes)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}
	}

	parsedSrv, startupPlan, err := parseCueServer(ctx, ft.loader, loc, server)
	if err != nil {
		return nil, fnerrors.Wrapf(loc, err, "parsing server")
	}

	parsedSrv.Volumes = append(parsedSrv.Volumes, parsedVolumes...)
	parsedSrv.Secret = parsedSecrets

	parsedPkg.Server, err = workspace.TransformOpaqueServer(ctx, ft.loader, loc, parsedSrv, parsedPkg, opts)
	if err != nil {
		return nil, err
	}

	if requires := v.LookupPath("requires"); requires.Exists() {
		var bits []schema.PackageName
		if err := requires.Val.Decode(&bits); err != nil {
			return nil, err
		}

		phase1plan.declaredStack = bits

		for _, p := range phase1plan.declaredStack {
			err := workspace.Ensure(ctx, ft.loader, p)
			if err != nil {
				return nil, fnerrors.Wrapf(loc, err, "loading package %s", p)
			}
		}
	}

	phase1plan.startupPlan = startupPlan

	return parsedPkg, nil
}

type cueSecret struct {
	Description string `json:"description,omitempty"`
}

func parseSecret(ctx context.Context, loc workspace.Location, name string, v cue.Value) (*schema.SecretSpec, error) {
	var bits cueSecret
	if err := v.Decode(&bits); err != nil {
		return nil, err
	}

	return &schema.SecretSpec{
		Owner:       loc.PackageName.String(),
		Name:        name,
		Description: bits.Description,
	}, nil
}
