// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/engine/ops"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/deploy/render"
	deploystorage "namespacelabs.dev/foundation/provision/deploy/storage"
	"namespacelabs.dev/foundation/provision/deploy/view"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func NewDeployCmd() *cobra.Command {
	var (
		explain       bool
		serializePath string
		deployOpts    deployOpts
		env           planning.Context
		locs          fncobra.Locations
		servers       fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "deploy [path/to/server]...",
			Short: "Deploy one, or more servers to the specified environment.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&deployOpts.alsoWait, "wait", true, "Wait for the deployment after running.")
			flags.BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.BoolVar(&runtime.NamingNoTLS, "naming_no_tls", runtime.NamingNoTLS, "If set to true, no TLS certificate is requested for ingress names.")
			flags.Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
			flags.StringVar(&serializePath, "serialize_to", "", "If set, rather than execute on the plan, output a serialization of the plan.")
			flags.StringVar(&deployOpts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			cluster, err := runtime.NamespaceFor(ctx, env)
			if err != nil {
				return err
			}

			stack, err := stack.Compute(ctx, servers.Servers, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
			if err != nil {
				return err
			}

			plan, err := deploy.PrepareDeployStack(ctx, env, cluster.Planner(), stack, servers.Servers)
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

			deployPlan := deploy.Serialize(env.Workspace().Proto(), env.Environment(), stack.Proto(), computed, provision.ServerPackages(servers.Servers).PackageNamesAsString())

			if serializePath != "" {
				return protos.WriteFile(serializePath, deployPlan)
			}

			sealed := pkggraph.MakeSealedContext(env, servers.SealedPackages)

			return completeDeployment(ctx, sealed, cluster, computed.Deployer, deployPlan, deployOpts)
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

func completeDeployment(ctx context.Context, env planning.Context, cluster runtime.ClusterNamespace, p *ops.Plan, plan *schema.DeployPlan, opts deployOpts) error {
	if err := orchestration.Deploy(ctx, env, cluster, p, plan, opts.alsoWait, true); err != nil {
		return err
	}

	var ports []*deploystorage.PortFwd
	for _, endpoint := range plan.Stack.Endpoint {
		ports = append(ports, &deploystorage.PortFwd{
			Endpoint: endpoint,
		})
	}

	out := console.TypedOutput(ctx, "deploy", console.CatOutputUs)

	var focusServer []*schema.Server
	for _, focus := range plan.FocusServer {
		if srv := plan.Stack.GetServer(schema.PackageName(focus)); srv != nil {
			focusServer = append(focusServer, srv.Server)
		}
	}

	r := deploystorage.ToStorageNetworkPlan("", plan.Stack, focusServer, ports, plan.IngressFragment)
	if r != nil {
		summary := render.NetworkPlanToSummary(r)
		view.NetworkPlanToText(out, summary, &view.NetworkPlanToTextOpts{
			Style:                 colors.Ctx(ctx),
			Checkmark:             false,
			IncludeSupportServers: true,
		})

		storedrun.Attach(r)
	}

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

		if err := os.WriteFile(opts.outputPath, serialized, 0644); err != nil {
			return fnerrors.New("failed to write %q: %w", opts.outputPath, err)
		}
	}

	envLabel := fmt.Sprintf("--env=%s", env.Environment().Name)

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

		if env.Environment().Purpose == schema.Environment_DEVELOPMENT {
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
