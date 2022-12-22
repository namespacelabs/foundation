// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"fmt"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
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

var packageFields = []string{
	"server", "sidecars", "volumes", "secrets", "tests", "binary",
	"resources", "resourceClasses", "providers",
}

var sidecarFields = []string{
	"args", "env", "mounts", "image", "imageFrom", "init",
}

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

	if err := cuefrontend.ValidateNoExtraFields(loc, "top level" /* messagePrefix */, v, packageFields); err != nil {
		return nil, err
	}

	parsedPkg, err := cuefrontend.ParsePackage(ctx, ft.env, ft.loader, v, loc)
	if err != nil {
		return nil, err
	}

	var validators []func() error

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, startupPlan, err := parseCueServer(ctx, ft.env, ft.loader, parsedPkg, server)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing server failed: %w", err)
		}

		// Defer validating the startup plan until the rest of the package is loaded.
		validators = append(validators, func() error {
			return validateStartupPlan(ctx, ft.loader, parsedPkg, startupPlan)
		})

		parsedSrv.Volume = append(parsedSrv.Volume, parsedPkg.Volumes...)

		var parsedSidecars []*schema.Container
		var parsedInitContainers []*schema.Container
		if sidecars := v.LookupPath("sidecars"); sidecars.Exists() {
			it, err := sidecars.Val.Fields()
			if err != nil {
				return nil, err
			}

			for it.Next() {
				val := &fncue.CueV{Val: it.Value()}

				if err := cuefrontend.ValidateNoExtraFields(loc, fmt.Sprintf("sidecar %q:", it.Label()) /* messagePrefix */, val, sidecarFields); err != nil {
					return nil, err
				}

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

		if naming := server.LookupPath("unstable_naming"); naming.Exists() {
			phase1plan.naming, err = cuefrontend.ParseNaming(naming)
			if err != nil {
				return nil, err
			}
		}

		parsedPkg.Parsed = phase1plan
	}

	parsedPkg.NewFrontend = true
	parsedPkg.PackageSources = partial.Package.Snapshot

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return parsedPkg, multierr.New(errs...)
}
