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
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
)

type Frontend struct {
	loader workspace.EarlyPackageLoader
}

func NewFrontend(pl workspace.EarlyPackageLoader) *Frontend {
	return &Frontend{loader: pl}
}

func (ft Frontend) ParsePackage(ctx context.Context, partial *fncue.Partial, loc pkggraph.Location, opts workspace.LoadPackageOpts) (*pkggraph.Package, error) {
	v := &partial.CueV

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	phase1plan := &phase1plan{}
	parsedPkg := &pkggraph.Package{
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

	var parsedSidecars []*schema.SidecarContainer
	var parsedInitContainers []*schema.SidecarContainer
	if sidecars := v.LookupPath("sidecars"); sidecars.Exists() {
		it, err := sidecars.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedContainer, err := parseCueContainer(ctx, ft.loader, it.Label(), loc, val)
			if err != nil {
				return nil, err
			}

			parsedVolumes = append(parsedVolumes, parsedContainer.inlineVolumes...)
			parsedPkg.Binaries = append(parsedPkg.Binaries, parsedContainer.inlineBinaries...)

			if v, _ := val.LookupPath("init").Val.Bool(); v {
				parsedInitContainers = append(parsedInitContainers, parsedContainer.container)
			} else {
				parsedSidecars = append(parsedSidecars, parsedContainer.container)
			}
		}
	}

	parsedSrv, startupPlan, err := parseCueServer(ctx, ft.loader, loc, server)
	if err != nil {
		return nil, fnerrors.Wrapf(loc, err, "parsing server")
	}

	if parsedSrv.RunByDefault {
		test, err := workspace.CreateServerStartupTest(ctx, ft.loader, loc.PackageName)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "creating server startup test")
		}
		parsedPkg.Tests = append(parsedPkg.Tests, test)
	}

	parsedSrv.Volumes = append(parsedSrv.Volumes, parsedVolumes...)
	parsedSrv.Secret = parsedSecrets

	parsedPkg.Server = parsedSrv

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

	if integration := server.LookupPath("integration"); integration.Exists() {
		if err := parseIntegration(ctx, loc, integration, parsedPkg); err != nil {
			return nil, err
		}
	}

	phase1plan.startupPlan = startupPlan
	phase1plan.sidecars = parsedSidecars
	phase1plan.initContainers = parsedInitContainers

	return parsedPkg, nil
}

type cueSecret struct {
	Description string `json:"description,omitempty"`
}

func parseSecret(ctx context.Context, loc pkggraph.Location, name string, v cue.Value) (*schema.SecretSpec, error) {
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
