// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package debug

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/std/cfg"
)

func newPrintSealedCmd() *cobra.Command {
	var (
		outputType  string = "json"
		printDeploy bool
		env         cfg.Context
		locs        fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "print-sealed",
			Short: "Load a server definition and print it's computed sealed workspace as JSON.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&outputType, "output", outputType, "One of json, textproto.")
			flags.BoolVar(&printDeploy, "deploy_stack", false, "If specified, prints the sealed workspace for each of the servers in the stack.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{RequireSingle: true})).
		Do(func(ctx context.Context) error {
			pl := parsing.NewPackageLoader(env)
			loc := locs.Locs[0]

			cluster, err := runtime.PlannerFor(ctx, env)
			if err != nil {
				return err
			}

			if !printDeploy {
				sealed, err := parsing.Seal(ctx, pl, loc.AsPackageName(), nil)
				if err != nil {
					return err
				}

				return output(ctx, pl, sealed.Proto, outputType)
			} else {
				t, err := planning.RequireServer(ctx, env, loc.AsPackageName())
				if err != nil {
					return err
				}

				plan, err := deploy.PrepareDeployServers(ctx, env, t.SealedContext(), cluster, t)
				if err != nil {
					return err
				}

				computedPlan, err := compute.GetValue(ctx, plan)
				if err != nil {
					return err
				}

				stack := computedPlan.ComputedStack.Proto()

				for _, s := range stack.Entry {
					if err := output(ctx, t.SealedContext(), s, outputType); err != nil {
						return err
					}
				}

				for _, endpoint := range stack.Endpoint {
					if err := output(ctx, t.SealedContext(), endpoint, outputType); err != nil {
						return err
					}
				}

				for _, endpoint := range stack.InternalEndpoint {
					if err := output(ctx, t.SealedContext(), endpoint, outputType); err != nil {
						return err
					}
				}

				for _, ingress := range computedPlan.IngressFragments {
					if err := output(ctx, t.SealedContext(), ingress, outputType); err != nil {
						return err
					}
				}

				return nil
			}
		})
}
