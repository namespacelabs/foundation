// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	integrationapplying "namespacelabs.dev/foundation/workspace/integration/api"
)

type Frontend struct {
	loader workspace.EarlyPackageLoader
	env    *schema.Environment
}

func NewFrontend(env *schema.Environment, pl workspace.EarlyPackageLoader) *Frontend {
	return &Frontend{pl, env}
}

func (ft Frontend) ParsePackage(ctx context.Context, partial *fncue.Partial, loc pkggraph.Location, opts workspace.LoadPackageOpts) (*pkggraph.Package, error) {
	v := &partial.CueV

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	parsedPkg := &pkggraph.Package{
		Location: loc,
	}

	if tests := v.LookupPath("tests"); tests.Exists() {
		parsedTests, err := parseTests(ctx, ft.env, ft.loader, loc, tests)
		if err != nil {
			return nil, err
		}

		parsedPkg.Tests = append(parsedPkg.Tests, parsedTests...)
	}

	if r := v.LookupPath("resourceClasses"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedRc, err := parseResourceClass(ctx, loc, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceClassSpecs = append(parsedPkg.ResourceClassSpecs, parsedRc)
		}
	}

	if r := v.LookupPath("providers"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}

			parsedProvider, err := parseResourceProvider(ctx, loc, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceProviders = append(parsedPkg.ResourceProviders, parsedProvider)
		}
	}

	if r := v.LookupPath("resources"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedResource, err := parseResourceInstance(ctx, ft.loader, loc, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceInstanceSpecs = append(parsedPkg.ResourceInstanceSpecs, parsedResource)
		}
	}

	server := v.LookupPath("server")
	if server.Exists() {
		parsedSrv, startupPlan, err := parseCueServer(ctx, ft.loader, loc, server)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing server")
		}

		if volumes := v.LookupPath("volumes"); volumes.Exists() {
			parsedVolumes, err := cuefrontend.ParseVolumes(ctx, ft.loader, loc, volumes)
			if err != nil {
				return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
			}
			parsedSrv.Volumes = append(parsedSrv.Volumes, parsedVolumes...)
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
				parsedContainer, err := parseCueContainer(ctx, ft.env, ft.loader, parsedPkg, it.Label(), loc, val)
				if err != nil {
					return nil, err
				}

				parsedSrv.Volumes = append(parsedSrv.Volumes, parsedContainer.inlineVolumes...)
				parsedPkg.Binaries = append(parsedPkg.Binaries, parsedContainer.inlineBinaries...)

				if v, _ := val.LookupPath("init").Val.Bool(); v {
					parsedInitContainers = append(parsedInitContainers, parsedContainer.container)
				} else {
					parsedSidecars = append(parsedSidecars, parsedContainer.container)
				}
			}
		}

		if secrets := v.LookupPath("secrets"); secrets.Exists() {
			parsedSrv.Secret, err = parseSecrets(ctx, loc, secrets)
			if err != nil {
				return nil, err
			}
		}

		parsedPkg.Server = parsedSrv

		if i := server.LookupPath("integration"); i.Exists() {
			integration, err := integrationparsing.IntegrationParser.ParseEntity(ctx, ft.loader, loc, i)
			if err != nil {
				return nil, err
			}

			parsedPkg.Integration = &schema.PackageIntegration{
				Data: protos.WrapAnyOrDie(integration.Data),
			}
		}

		if image := server.LookupPath("image"); image.Exists() {
			bin, err := ParseImage(ctx, loc, image)
			if err != nil {
				return nil, err
			}

			// TODO: don't set the server binary here, instead introduce an "image" integration.
			err = integrationapplying.SetServerBinary(parsedPkg, bin, nil)
			if err != nil {
				return nil, err
			}
		}

		phase1plan := &phase1plan{
			startupPlan:    startupPlan,
			sidecars:       parsedSidecars,
			initContainers: parsedInitContainers,
		}
		if requires := v.LookupPath("requires"); requires.Exists() {
			phase1plan.declaredStack, err = parseRequires(ctx, ft.loader, loc, requires)
			if err != nil {
				return nil, err
			}
		}
		parsedPkg.Parsed = phase1plan
	}

	return parsedPkg, nil
}
