// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func NewDeployCmd() *cobra.Command {
	var (
		usePackageNames bool
		runOpts         deploy.Opts
		explain         bool
		serializePath   string
		deployOpts      deployOpts
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy one, or more servers to the specified environment.",
		Args:  cobra.ArbitraryArgs,
	}

	cmd.Flags().BoolVar(&usePackageNames, "use_package_names", usePackageNames, "Specify servers by using their fully qualified package name instead.")
	cmd.Flags().Int32Var(&runOpts.BaseServerPort, "port_base", 40000, "Base port to listen on (additional requested ports will be base port + n).")
	cmd.Flags().BoolVar(&deployOpts.alsoWait, "wait", true, "Wait for the deployment after running.")
	cmd.Flags().BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
	cmd.Flags().BoolVar(&runtime.NamingNoTLS, "naming_no_tls", runtime.NamingNoTLS, "If set to true, no TLS certificate is requested for ingress names.")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
	cmd.Flags().StringVar(&serializePath, "serialize_to", "", "If set, rather than execute on the plan, output a serialization of the plan.")
	cmd.Flags().StringVar(&deployOpts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		locations, specified, err := allServersOrFromArgs(ctx, env, usePackageNames, args)
		if err != nil {
			return err
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

		deployPlan := deploy.Serialize(env.Workspace(), env.Proto(), stack.Proto(), computed, provision.ServerPackages(servers).PackageNamesAsString())

		if serializePath != "" {
			return protos.WriteFile(serializePath, deployPlan)
		}

		return completeDeployment(ctx, env.BindWith(packages), computed.Deployer, deployPlan, deployOpts)
	})
}

type deployOpts struct {
	alsoWait   bool
	outputPath string
}

type Output struct {
	Ingress []Ingress `json:"ingress"`
}

type Ingress struct {
	Owner    string   `json:"owner"`
	Fdqn     string   `json:"fdqn"`
	Protocol []string `json:"protocol"`
}

func completeDeployment(ctx context.Context, env ops.Environment, p *ops.Plan, plan *schema.DeployPlan, opts deployOpts) error {
	waiters, err := p.Execute(ctx, runtime.TaskServerDeploy, env)
	if err != nil {
		return err
	}

	if opts.alsoWait {
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

	domains := runtime.FilterAndDedupDomains(plan.IngressFragment, nil)

	out := console.TypedOutput(ctx, "deploy", console.CatOutputUs)

	var focusServer []*schema.Server
	for _, focus := range plan.FocusServer {
		if srv := plan.Stack.GetServer(schema.PackageName(focus)); srv != nil {
			focusServer = append(focusServer, srv.Server)
		}
	}

	deploy.SortPorts(ports, focusServer)
	deploy.SortIngresses(plan.IngressFragment)
	r := deploy.RenderPortsAndIngresses("", plan.Stack, focusServer, ports, domains, plan.IngressFragment)
	deploy.RenderText(out, colors.Ctx(ctx), r, false, "")

	storedrun.Attach(r)

	if opts.outputPath != "" {
		var out Output
		for _, frag := range plan.IngressFragment {
			ingress := Ingress{
				Owner: frag.Owner,
				Fdqn:  frag.Domain.Fqdn,
			}

			var protocols uniquestrings.List
			for _, md := range frag.GetEndpoint().GetServiceMetadata() {
				if md.Protocol != "" {
					protocols.Add(md.Protocol)
				}
			}
			ingress.Protocol = protocols.Strings()

			out.Ingress = append(out.Ingress, ingress)
		}
		serialized, err := json.MarshalIndent(out, "", " ")
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(opts.outputPath, serialized, 0644); err != nil {
			return fnerrors.New("failed to write %q: %w", opts.outputPath, err)
		}
	}

	envLabel := fmt.Sprintf("--env=%s", env.Proto().Name)

	fmt.Fprintf(out, "\n Next steps:\n\n")

	var hints []string
	for _, pkg := range plan.FocusServer {
		srv := plan.Stack.GetServer(schema.PackageName(pkg))
		if srv == nil {
			fmt.Fprintf(console.Debug(ctx), "%s: missing from the stack\n", pkg)
			continue
		}

		var loc string
		if plan.GetWorkspace().GetModuleName() == srv.Server.ModuleName {
			if x, ok := fnfs.ResolveLocation(srv.Server.ModuleName, srv.Server.PackageName); ok {
				loc = x.RelPath
			}
		}

		if loc == "" {
			loc = fmt.Sprintf("--use_package_names %s", srv.GetPackageName())
		}

		highlight := colors.Ctx(ctx).Highlight
		hints = append(hints, fmt.Sprintf("Tail server logs: %s", highlight.Apply(fmt.Sprintf("ns logs %s %s", envLabel, loc))))
		hints = append(hints, fmt.Sprintf("Attach to the deployment (port forward to workstation): %s", highlight.Apply(fmt.Sprintf("ns attach %s %s", envLabel, loc))))

		if env.Proto().Purpose == schema.Environment_DEVELOPMENT {
			hints = append(hints, fmt.Sprintf("Try out a stateful development session with %s.",
				highlight.Apply(fmt.Sprintf("ns dev %s %s", envLabel, loc))))
		}
	}

	hints = append(hints, plan.Hints...)
	for _, hint := range hints {
		fmt.Fprintf(out, "   · %s\n", hint)
	}

	return nil
}
