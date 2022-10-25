// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	integrationapplying "namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Frontend struct {
	loader parsing.EarlyPackageLoader
	env    *schema.Environment
}

func NewFrontend(env *schema.Environment, pl parsing.EarlyPackageLoader) *Frontend {
	return &Frontend{pl, env}
}

func (ft Frontend) ParsePackage(ctx context.Context, partial *fncue.Partial, loc pkggraph.Location) (*pkggraph.Package, error) {
	v := &partial.CueV

	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	parsedPkg := &pkggraph.Package{
		Location: loc,
	}

	if tests := v.LookupPath("tests"); tests.Exists() {
		parsedTests, err := parseTests(ctx, ft.env, ft.loader, parsedPkg, tests)
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

			parsedProvider, err := parseResourceProvider(ctx, ft.env, ft.loader, parsedPkg, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceProvidersSpecs = append(parsedPkg.ResourceProvidersSpecs, parsedProvider)
		}
	}

	if r := v.LookupPath("resources"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedResource, err := cuefrontend.ParseResourceInstanceFromCue(ctx, ft.env, ft.loader, parsedPkg, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceInstanceSpecs = append(parsedPkg.ResourceInstanceSpecs, parsedResource)
		}
	}

	if volumes := v.LookupPath("volumes"); volumes.Exists() {
		parsedVolumes, err := cuefrontend.ParseVolumes(ctx, ft.loader, loc, volumes)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing volumes")
		}
		parsedPkg.Volumes = append(parsedPkg.Volumes, parsedVolumes...)
	}

	if secrets := v.LookupPath("secrets"); secrets.Exists() {
		secretSpecs, err := parseSecrets(ctx, secrets)
		if err != nil {
			return nil, err
		}

		parsedPkg.Secrets = append(parsedPkg.Secrets, secretSpecs...)
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, startupPlan, err := parseCueServer(ctx, ft.env, ft.loader, parsedPkg, server)
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "parsing server")
		}

		parsedSrv.Volume = append(parsedSrv.Volume, parsedPkg.Volumes...)

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

				parsedSrv.Volume = append(parsedSrv.Volume, parsedContainer.volumes...)
				parsedPkg.Binaries = append(parsedPkg.Binaries, parsedContainer.inlineBinaries...)

				if v, _ := val.LookupPath("init").Val.Bool(); v {
					parsedInitContainers = append(parsedInitContainers, parsedContainer.container)
				} else {
					parsedSidecars = append(parsedSidecars, parsedContainer.container)
				}
			}
		}

		parsedPkg.Server = parsedSrv

		if i := server.LookupPath("integration"); i.Exists() {
			integration, err := integrationparsing.IntegrationParser.ParseEntity(ctx, ft.env, ft.loader, loc, i)
			if err != nil {
				return nil, err
			}

			parsedPkg.Integration = &schema.Integration{
				Data: protos.WrapAnyOrDie(integration.Data),
			}
		}

		binRef, err := binary.ParseImage(ctx, ft.env, ft.loader, parsedPkg, "server" /* binaryName */, server, binary.ParseImageOpts{})
		if err != nil {
			return nil, err
		}
		if binRef != nil {
			if err := integrationapplying.SetServerBinaryRef(parsedPkg, binRef); err != nil {
				return nil, err
			}
		}

		phase1plan := &phase1plan{
			startupPlan:    startupPlan,
			sidecars:       parsedSidecars,
			initContainers: parsedInitContainers,
		}
		if requires := server.LookupPath("requires"); requires.Exists() {
			phase1plan.declaredStack, err = parseRequires(ctx, ft.loader, loc, requires)
			if err != nil {
				return nil, err
			}
		}

		parsedPkg.Parsed = phase1plan
	}

	parsedPkg.NewFrontend = true
	parsedPkg.PackageSources = partial.Package.Snapshot

	return parsedPkg, nil
}
