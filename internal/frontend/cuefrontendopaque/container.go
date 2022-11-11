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
	Env  map[string]string   `json:"env"`
}

type parsedCueContainer struct {
	container      *schema.SidecarContainer
	volumes        []*schema.Volume
	inlineBinaries []*schema.Binary
}

// TODO: make it common for the main "server" container and sidecars.
func parseCueContainer(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, name string, loc pkggraph.Location, v *fncue.CueV) (*parsedCueContainer, error) {
	var bits cueContainer
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	out := &parsedCueContainer{
		container: &schema.SidecarContainer{
			Name: name,
			Args: bits.Args.Parsed(),
		},
	}

	for k, v := range bits.Env {
		out.container.Env = append(out.container.Env, &schema.BinaryConfig_EnvEntry{
			Name: k, Value: v,
		})
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

	var err error
	out.container.BinaryRef, err = binary.ParseImage(ctx, env, pl, pkg, name, v, binary.ParseImageOpts{Required: true})
	if err != nil {
		return nil, fnerrors.NewWithLocation(loc, "parsing container %q failed: %w", name, err)
	}

	return out, nil
}
