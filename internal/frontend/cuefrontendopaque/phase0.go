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

	allowedFields := packageFields
	if loc.Module.ModuleName() == string(loc.PackageName) {
		allowedFields = append(allowedFields, cuefrontend.ModuleFields...)
	}

	// Is this too strict? What if there is non-Namespace CUE in a package?
	if err := cuefrontend.ValidateNoExtraFields(loc, "top level" /* messagePrefix */, v, allowedFields); err != nil {
		return nil, err
	}

	parsedPkg, err := cuefrontend.ParsePackage(ctx, ft.env, ft.loader, v, loc)
	if err != nil {
		return nil, err
	}

	if server := v.LookupPath("server"); server.Exists() {
		parsedSrv, err := parseCueServer(ctx, ft.env, ft.loader, parsedPkg, server)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing server failed: %w", err)
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
		} else if binRef != nil {
			if err := integrationapplying.SetServerBinaryRef(parsedPkg, binRef); err != nil {
				return nil, err
			}
		}

		if naming := server.LookupPath("unstable_naming"); naming.Exists() {
			parsedPkg.Server.ServerNaming, err = cuefrontend.ParseNaming(naming)
			if err != nil {
				return nil, err
			}
		}
	}

	if extension := v.LookupPath("extension"); extension.Exists() {
		parsed, err := parseCueServerExtension(ctx, ft.env, ft.loader, parsedPkg, extension)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing server failed: %w", err)
		}

		if parsedPkg.Server != nil {
			return nil, fnerrors.NewWithLocation(loc, "it is not yet possible to declare a server and an extension in the same package")
		}

		parsedPkg.ServerFragment = parsed
	}

	parsedPkg.NewFrontend = true
	parsedPkg.PackageSources = partial.Package.Snapshot

	return parsedPkg, nil
}
