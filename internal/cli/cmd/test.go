// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/morikuni/aec"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func NewTestCmd() *cobra.Command {
	var (
		runOpts        deploy.Opts
		testOpts       testing.TestOpts
		includeServers bool
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run a functional end-to-end test.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			devEnv, err := provision.RequireEnv(root, "dev")
			if err != nil {
				return err
			}

			var locs []fnfs.Location

			if len(args) == 0 {
				list, err := workspace.ListSchemas(ctx, root)
				if err != nil {
					return err
				}

				pl := workspace.NewPackageLoader(root)
				for _, l := range list.Locations {
					pp, err := pl.LoadByName(ctx, l.AsPackageName())
					if err != nil {
						return err
					}

					// We also automatically generate a startup-test for each server.
					if pp.Test != nil || (includeServers && pp.Server != nil) {
						locs = append(locs, l)
					}
				}
			} else {
				for _, arg := range args {
					loc := root.RelPackage(arg)
					locs = append(locs, loc)
				}
			}

			stderr := console.Stderr(ctx)
			pl := workspace.NewPackageLoader(devEnv.Root())

			for _, loc := range locs {
				// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
				env := testing.PrepareEnvFrom(devEnv)

				test, err := testing.PrepareTest(ctx, pl, env, loc.AsPackageName(), testOpts, func(ctx context.Context, pl *workspace.PackageLoader, test *schema.Test) ([]provision.Server, *stack.Stack, error) {
					var suts []provision.Server

					for _, pkg := range test.ServersUnderTest {
						sut, err := env.RequireServerWith(ctx, pl, schema.PackageName(pkg))
						if err != nil {
							return nil, nil, err
						}
						suts = append(suts, sut)
					}

					stack, err := stack.Compute(ctx, suts, stack.ProvisionOpts{PortBase: runOpts.BaseServerPort})
					if err != nil {
						return nil, nil, err
					}

					return suts, stack, nil
				})

				if err != nil {
					return err
				}

				v, err := compute.Get(ctx, test)
				if err != nil {
					return err
				}

				status := aec.GreenF.Apply("PASSED")
				if !v.Value.Bundle.Result.Success {
					status = aec.RedF.Apply("FAILED")
				}

				cached := ""
				if v.Cached {
					cached = aec.LightBlackF.Apply(" (CACHED)")
				}

				fmt.Fprintf(stderr, "%s: Test %s%s %s\n", loc.AsPackageName(), status, cached, aec.LightBlackF.Apply(v.Value.ImageRef.ImageRef()))
			}

			return nil
		}),
	}

	cmd.Flags().Int32Var(&runOpts.BaseServerPort, "port_base", 40000, "Base port to listen on (additional requested ports will be base port + n).")
	cmd.Flags().BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
	cmd.Flags().BoolVar(&testOpts.KeepRuntime, "keep_runtime", testOpts.KeepRuntime, "If true, don't cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
	cmd.Flags().BoolVar(&includeServers, "include_servers", includeServers, "If true, also include generated server startup-tests.")

	return cmd
}
