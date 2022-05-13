// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func NewDeployCmd() *cobra.Command {
	const defaultEnvRef = "dev"

	var (
		envRef      = defaultEnvRef
		packageName string
		runOpts     deploy.Opts
		alsoWait    = true
		explain     bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy one, or more servers to the specified environment.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			env, err := requireEnv(ctx, envRef)
			if err != nil {
				return err
			}

			var locations []fnfs.Location
			var specified bool
			if packageName != "" {
				loc, err := workspace.NewPackageLoader(env.Root()).Resolve(ctx, schema.PackageName(packageName))
				if err != nil {
					return err
				}

				locations = append(locations, fnfs.Location{
					ModuleName: loc.Module.ModuleName(),
					RelPath:    loc.Rel(),
				})
				specified = true
			} else {
				locations, specified, err = allServersOrFromArgs(ctx, env, args)
				if err != nil {
					return err
				}
			}

			packages, servers, err := loadServers(ctx, env, locations, specified)
			if err != nil {
				return err
			}

			stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortBase: runOpts.BaseServerPort})
			if err != nil {
				return err
			}

			plan, err := deploy.PrepareDeployStack(ctx, env, stack, servers)
			if err != nil {
				return err
			}

			if explain {
				return compute.Explain(ctx, console.Stdout(ctx), plan)
			}

			computed, err := compute.GetValue(ctx, plan)
			if err != nil {
				return err
			}

			waiters, err := computed.Deployer.Execute(ctx, runtime.TaskServerDeploy, env.BindWith(packages))
			if err != nil {
				return err
			}

			if alsoWait {
				if err := deploy.Wait(ctx, env, waiters); err != nil {
					return err
				}
			}

			var focusServers []*schema.Server
			for _, srv := range servers {
				focusServers = append(focusServers, srv.Proto())
			}

			var ports []*deploy.PortFwd
			for _, endpoint := range stack.Endpoints {
				ports = append(ports, &deploy.PortFwd{
					Endpoint: endpoint,
				})
			}

			domains, err := runtime.FilterAndDedupDomains(computed.IngressFragments, nil)
			if err != nil {
				domains = nil
				fmt.Fprintln(console.Stderr(ctx), "Failed to report on ingress:", err)
			}

			out := console.TypedOutput(ctx, "deploy", console.CatOutputUs)

			deploy.SortPorts(ports, focusServers)
			deploy.SortIngresses(computed.IngressFragments)
			deploy.RenderPortsAndIngresses(false, out, "", stack.Proto(),
				focusServers, ports, domains, computed.IngressFragments)

			envLabel := fmt.Sprintf("--env=%s ", envRef)
			if envRef == defaultEnvRef {
				envLabel = ""
			}

			fmt.Fprintf(out, "\n Next steps:\n\n")

			for _, srv := range servers {
				var hints []string
				hints = append(hints, fmt.Sprintf("Tail server logs: %s", colors.Bold(fmt.Sprintf("fn logs %s%s", envLabel, srv.Location.Rel()))))
				hints = append(hints, fmt.Sprintf("Attach to the deployment (port forward to workstation): %s", colors.Bold(fmt.Sprintf("fn attach %s%s", envLabel, srv.Location.Rel()))))
				hints = append(hints, computed.Hints...)

				if env.Purpose() == schema.Environment_DEVELOPMENT {
					hints = append(hints, fmt.Sprintf("Try out a stateful development session with %s.",
						colors.Bold(fmt.Sprintf("fn dev %s%s", envLabel, srv.Location.Rel()))))
				}

				for _, hint := range hints {
					fmt.Fprintf(out, "   · %s\n", hint)
				}
			}

			return nil
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to provision (as defined in the workspace).")
	cmd.Flags().StringVar(&packageName, "package_name", packageName, "Instead of running the specified local server, run the specified package resolved against the local workspace.")
	cmd.Flags().Int32Var(&runOpts.BaseServerPort, "port_base", 40000, "Base port to listen on (additional requested ports will be base port + n).")
	cmd.Flags().BoolVar(&alsoWait, "wait", alsoWait, "Wait for the deployment after running.")
	cmd.Flags().BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
	cmd.Flags().BoolVar(&runtime.NamingNoTLS, "naming_no_tls", runtime.NamingNoTLS, "If set to true, no TLS certificate is requested for ingress names.")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")

	return cmd
}
