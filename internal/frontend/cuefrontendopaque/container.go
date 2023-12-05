// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueContainer struct {
	Args *args.ArgsListOrMap `json:"args"`
	Env  *args.EnvMap        `json:"env"`

	Requests *schema.Container_ResourceLimits `json:"resourceRequests"`
	Limits   *schema.Container_ResourceLimits `json:"resourceLimits"`
	Security *cueServerSecurity               `json:"security,omitempty"`
}

type parsedCueContainer struct {
	container      *schema.Container
	volumes        []*schema.Volume
	inlineBinaries []*schema.Binary
}

// TODO: make it common for the main "server" container and sidecars.
func parseCueContainer(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, name string, loc pkggraph.Location, v *fncue.CueV) (*parsedCueContainer, error) {
	var bits cueContainer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	envVars, err := bits.Env.Parsed(ctx, pl, loc)
	if err != nil {
		return nil, err
	}

	out := &parsedCueContainer{
		container: &schema.Container{
			Name:     name,
			Args:     bits.Args.Parsed(),
			Env:      envVars,
			Limits:   bits.Limits,
			Requests: bits.Requests,
		},
	}

	if bits.Security != nil {
		if err := parsing.RequireFeature(loc.Module, "experimental/container/security"); err != nil {
			return nil, fnerrors.AttachLocation(loc, err)
		}

		out.container.Security = &schema.Container_Security{
			Privileged:   bits.Security.Privileged,
			HostNetwork:  bits.Security.HostNetwork,
			Capabilities: bits.Security.Capabilities,
		}
	}

	if mounts := v.LookupPath("mounts"); mounts.Exists() {
		var err error
		out.container.Mount, out.volumes, err = cuefrontend.ParseMounts(ctx, pl, loc, mounts)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing volumes failed: %w", err)
		}

		// Not fully implemented yet, disabling for now.
		if len(out.container.Mount) != 0 {
			return nil, fnerrors.NewWithLocation(loc, "mounts are not supported for sidecar containers at the moment")
		}
	}

	out.container.BinaryRef, err = binary.ParseImage(ctx, env, pl, pkg, name, v, binary.ParseImageOpts{Required: true})
	if err != nil {
		return nil, fnerrors.NewWithLocation(loc, "parsing container %q failed: %w", name, err)
	}

	return out, nil
}
