// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

type hydrateParser struct {
	resultOut *hydrateResult

	env     *provision.Env
	servers *fncobra.Servers

	rehydrateOnly bool
	rehydrate     bool
}

type hydrateOpts struct {
	// If false, allows controlling whether to rehydrate via the "rehydrate" flag.
	rehydrateOnly bool
	// Default value of the flag.
	rehydrate bool
}

type hydrateResult struct {
	Env        runtime.Selector
	Stack      *schema.Stack
	Focus      []schema.PackageName
	Ingress    []*schema.IngressFragment
	Rehydrated *config.Rehydrated
}

func parseHydration(resultOut *hydrateResult, env *provision.Env, servers *fncobra.Servers, opts *hydrateOpts) *hydrateParser {
	return &hydrateParser{
		resultOut:     resultOut,
		env:           env,
		servers:       servers,
		rehydrate:     opts.rehydrate,
		rehydrateOnly: opts.rehydrateOnly,
	}
}

// Initializes parseHydration() with its dependencies.
func parseHydrationWithDeps(resultOut *hydrateResult, locationsOpts *fncobra.ParseLocationsOpts, opts *hydrateOpts) []fncobra.ArgParser {
	var (
		env     provision.Env
		locs    fncobra.Locations
		servers fncobra.Servers
	)

	return []fncobra.ArgParser{
		fncobra.ParseEnv(&env),
		fncobra.ParseLocations(&locs, locationsOpts),
		fncobra.ParseServers(&servers, &env, &locs),
		parseHydration(resultOut, &env, &servers, opts),
	}
}

func (h *hydrateParser) AddFlags(cmd *cobra.Command) {
	if !h.rehydrateOnly {
		cmd.Flags().BoolVar(&h.rehydrate, "rehydrate", h.rehydrate, "If set to false, compute stack at head, rather than loading the deployed configuration.")
	}
}

func (h *hydrateParser) Parse(ctx context.Context, args []string) error {
	if h.resultOut == nil {
		return fnerrors.InternalError("resultOut must be set")
	}
	if h.servers == nil {
		return fnerrors.InternalError("servers must be set")
	}
	if h.env == nil {
		return fnerrors.InternalError("env must be set")
	}

	servers := h.servers.Servers

	for _, srv := range servers {
		h.resultOut.Focus = append(h.resultOut.Focus, srv.PackageName())
	}

	h.resultOut.Env = h.env

	if h.rehydrate || h.rehydrateOnly {
		if len(servers) != 1 {
			return fnerrors.UserError(nil, "--rehydrate only supports a single server")
		}

		buildID, err := runtime.For(ctx, h.env).DeployedConfigImageID(ctx, servers[0].Proto())
		if err != nil {
			return err
		}

		rehydrated, err := config.Rehydrate(ctx, servers[0], buildID)
		if err != nil {
			return err
		}

		h.resultOut.Stack = rehydrated.Stack
		h.resultOut.Ingress = rehydrated.IngressFragments
		h.resultOut.Rehydrated = rehydrated
	} else {
		stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
		if err != nil {
			return err
		}

		h.resultOut.Stack = stack.Proto()
		for _, entry := range stack.Proto().Entry {
			deferred, err := runtime.ComputeIngress(ctx, h.env.Proto(), entry, stack.Endpoints)
			if err != nil {
				return err
			}

			h.resultOut.Ingress = deferred
		}
	}

	return nil
}
