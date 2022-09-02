// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Servers struct {
	Servers        []provision.Server
	SealedPackages workspace.SealedPackages
}

func ParseServers(serversOut *Servers, env *provision.Env, locs *Locations) *ServersParser {
	return &ServersParser{
		serversOut: serversOut,
		locs:       locs,
		env:        env,
	}
}

type ServersParser struct {
	serversOut *Servers
	locs       *Locations
	env        *provision.Env
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

	var servers []provision.Server
	pl := workspace.NewPackageLoaderFromEnv(p.env)
	for _, loc := range p.locs.Locs {
		if err := tasks.Action("package.load-server").Scope(loc.AsPackageName()).Run(ctx, func(ctx context.Context) error {
			pp, err := pl.LoadByName(ctx, loc.AsPackageName())
			if err != nil {
				return fnerrors.Wrap(loc, err)
			}

			if pp.Server == nil {
				if p.locs.AreSpecified {
					return fnerrors.UserError(loc, "expected a server")
				}

				return nil
			}

			server, err := p.env.RequireServerWith(ctx, pl, loc.AsPackageName())
			if err != nil {
				return err
			}

			// If the user doesn't explicitly specify this server should be loaded, don't load it, if it's tagged as being testonly.
			if !p.locs.AreSpecified && server.Package.Server.Testonly {
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
