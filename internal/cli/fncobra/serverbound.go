// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type Servers struct {
	Servers        planning.Servers
	SealedPackages pkggraph.SealedPackageLoader
}

func ParseServers(serversOut *Servers, env *cfg.Context, locs *Locations) *ServersParser {
	return &ServersParser{
		serversOut: serversOut,
		locs:       locs,
		env:        env,
	}
}

type ServersParser struct {
	serversOut *Servers
	locs       *Locations
	env        *cfg.Context
}

func (p *ServersParser) AddFlags(cmd *cobra.Command) {}

func (p *ServersParser) Parse(ctx context.Context, args []string) error {
	if p.serversOut == nil {
		return fnerrors.InternalError("serversOut must be set")
	}
	if p.locs == nil {
		return fnerrors.InternalError("locs must be set")
	}
	if p.env == nil {
		return fnerrors.InternalError("env must be set")
	}

	var servers []planning.Server
	pl := parsing.NewPackageLoader(*p.env)
	for _, loc := range p.locs.Locations {
		if err := tasks.Action("package.load-server").Scope(loc.AsPackageName()).Run(ctx, func(ctx context.Context) error {
			pp, err := pl.LoadByName(ctx, loc.AsPackageName())
			if err != nil {
				return fnerrors.AttachLocation(loc, err)
			}

			if pp.Server == nil {
				if p.locs.UserSpecified {
					return fnerrors.NewWithLocation(loc, "expected a server")
				}

				return nil
			}

			server, err := planning.RequireServerWith(ctx, *p.env, pl, loc.AsPackageName())
			if err != nil {
				return err
			}

			if !p.locs.UserSpecified && !server.Package.Server.RunByDefault {
				return nil
			}

			servers = append(servers, server)
			return nil
		}); err != nil {
			return err
		}
	}

	*p.serversOut = Servers{
		Servers:        servers,
		SealedPackages: pl.Seal(),
	}

	return nil
}
