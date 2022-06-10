// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func NewDeployCmd() *cobra.Command {
	var (
		packageName   string
		runOpts       deploy.Opts
		alsoWait      = true
		explain       bool
		serializePath string
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy one, or more servers to the specified environment.",
		Args:  cobra.ArbitraryArgs,
	}

	cmd.Flags().StringVar(&packageName, "package_name", packageName, "Instead of running the specified local server, run the specified package resolved against the local workspace.")
	cmd.Flags().Int32Var(&runOpts.BaseServerPort, "port_base", 40000, "Base port to listen on (additional requested ports will be base port + n).")
	cmd.Flags().BoolVar(&alsoWait, "wait", alsoWait, "Wait for the deployment after running.")
	cmd.Flags().BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
	cmd.Flags().BoolVar(&runtime.NamingNoTLS, "naming_no_tls", runtime.NamingNoTLS, "If set to true, no TLS certificate is requested for ingress names.")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
	cmd.Flags().StringVar(&serializePath, "serialize_to", "", "If set, rather than execute on the plan, output a serialization of the plan.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
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
			var err error
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

		deployPlan := &schema.DeployPlan{
			Environment:     env.Proto(),
			Stack:           stack.Proto(),
			IngressFragment: computed.IngressFragments,
			Program:         computed.Deployer.Serialize(),
			Hints:           computed.Hints,
		}

		for _, srv := range servers {
			deployPlan.FocusServer = append(deployPlan.FocusServer, srv.Proto())
			deployPlan.RelLocation = append(deployPlan.RelLocation, srv.Location.Rel())
		}

		if serializePath != "" {
			serialized, err := proto.MarshalOptions{Deterministic: true}.Marshal(deployPlan)
			if err != nil {
				return fnerrors.New("failed to marshal: %w", err)
			}

			if err := ioutil.WriteFile(serializePath, serialized, 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", serializePath, err)
			}

			return nil
		}

		return completeDeployment(ctx, env.BindWith(packages), computed.Deployer, deployPlan, alsoWait)
	})
}

func completeDeployment(ctx context.Context, env ops.Environment, p *ops.Plan, plan *schema.DeployPlan, alsoWait bool) error {
	waiters, err := p.Execute(ctx, runtime.TaskServerDeploy, env)
	if err != nil {
		return err
	}

	if alsoWait {
		if err := deploy.Wait(ctx, env, waiters); err != nil {
			return err
		}
	}

	var ports []*deploy.PortFwd
	for _, endpoint := range plan.Stack.Endpoint {
		ports = append(ports, &deploy.PortFwd{
			Endpoint: endpoint,
		})
	}

	domains, err := runtime.FilterAndDedupDomains(plan.IngressFragment, nil)
	if err != nil {
		domains = nil
		fmt.Fprintln(console.Stderr(ctx), "Failed to report on ingress:", err)
	}

	out := console.TypedOutput(ctx, "deploy", console.CatOutputUs)

	deploy.SortPorts(ports, plan.FocusServer)
	deploy.SortIngresses(plan.IngressFragment)
	deploy.RenderPortsAndIngresses(false, out, "", plan.Stack, plan.FocusServer, ports, domains, plan.IngressFragment)

	envLabel := fmt.Sprintf("--env=%s ", env.Proto().Name)

	fmt.Fprintf(out, "\n Next steps:\n\n")

	for _, loc := range plan.RelLocation {
		var hints []string
		hints = append(hints, fmt.Sprintf("Tail server logs: %s", colors.Bold(fmt.Sprintf("fn logs %s%s", envLabel, loc))))
		hints = append(hints, fmt.Sprintf("Attach to the deployment (port forward to workstation): %s", colors.Bold(fmt.Sprintf("fn attach %s%s", envLabel, loc))))
		hints = append(hints, plan.Hints...)

		if env.Proto().Purpose == schema.Environment_DEVELOPMENT {
			hints = append(hints, fmt.Sprintf("Try out a stateful development session with %s.",
				colors.Bold(fmt.Sprintf("fn dev %s%s", envLabel, loc))))
		}

		for _, hint := range hints {
			fmt.Fprintf(out, "   · %s\n", hint)
		}
	}

	return nil
}
