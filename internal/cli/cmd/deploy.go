// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/planning/render"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build"
	buildr "namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/buildkite"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra/planningargs"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	deploystorage "namespacelabs.dev/foundation/internal/planning/deploy/storage"
	"namespacelabs.dev/foundation/internal/planning/deploy/view"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func NewDeployCmd() *cobra.Command {
	var (
		explain          bool
		uploadToRegistry bool
		serializePath    string
		uploadTo         string
		deployOpts       deployOpts
		env              cfg.Context
		locs             fncobra.Locations
		servers          planningargs.Servers
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
			flags.StringVar(&uploadTo, "upload_plan_to", "", "If set, rather than execute on the plan, upload a serialized version of the plan.")
			flags.BoolVar(&uploadToRegistry, "upload_to_registry", false, "If set, uploads the deploy plan to the cluster registry, instead of applying it.")
			flags.StringVar(&deployOpts.outputPath, "output_to", "", "If set, a machine-readable output is emitted after successful deployment.")
			flags.StringVar(&deployOpts.manualReason, "reason", "", "Why was this deployment triggered.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true}),
			planningargs.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			if deploy.RequireReason(env.Configuration()) && serializePath == "" && deployReason(deployOpts) == "" {
				return fnerrors.New("--reason is required when deploying to environment %q", env.Environment().Name)
			}

			p, err := planning.NewPlanner(ctx, env)
			if err != nil {
				return err
			}

			stack, err := planning.ComputeStack(ctx, servers.Servers, planning.ProvisionOpts{
				Planner:   p.Runtime,
				PortRange: eval.DefaultPortRange(),
			})
			if err != nil {
				return err
			}

			// When uploading a plan, any server and container images should be
			// pushed to the same repository, so they're accessible by the plan.

			var target compute.Computable[oci.RepositoryWithParent]
			if uploadTo != "" {
				if uploadToRegistry {
					target = p.Registry.AllocateName(uploadTo, "")
				} else {
					p.Registry = registry.MakeStaticRegistry(&buildr.Registry{
						Url: uploadTo,
					})
					target = registry.Precomputed(registry.AttachStaticKeychain(nil, filepath.Join(uploadTo, "plan"), oci.RegistryAccess{}))
					uploadToRegistry = true
				}
			}

			plan, err := deploy.PrepareDeployStack(ctx, p, stack)
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

			deployPlan := deploy.Serialize(env.Workspace().Proto(), env.Environment(), stack.Proto(), computed, servers.Servers.Packages())

			if serializePath != "" {
				any, err := anypb.New(deployPlan)
				if err != nil {
					return err
				}

				return protos.WriteFile(serializePath, any)
			}

			if target != nil {
				return uploadPlanTo(ctx, target, deployPlan)
			}

			sealed := pkggraph.MakeSealedContext(env, servers.SealedPackages)

			cluster, err := runtime.NamespaceFor(ctx, env)
			if err != nil {
				return err
			}

			return completeDeployment(ctx, sealed, cluster, deployPlan, deployOpts)
		})
}

type deployOpts struct {
	alsoWait     bool
	outputPath   string
	manualReason string
}

type Output struct {
	Ingress []Ingress `json:"ingress"`
}

type Ingress struct {
	Owner    string   `json:"owner"`
	Fdqn     string   `json:"fdqn"`
	Protocol []string `json:"protocol"`
}

func completeDeployment(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, plan *schema.DeployPlan, opts deployOpts) error {
	if err := orchestration.Deploy(ctx, env, cluster, plan, deployReason(opts), opts.alsoWait, true); err != nil {
		return err
	}

	var ports []*deploystorage.PortFwd
	for _, endpoint := range plan.Stack.Endpoint {
		ports = append(ports, &deploystorage.PortFwd{
			Endpoint: endpoint,
		})
	}

	out := console.TypedOutput(ctx, "deploy", console.CatOutputUs)

	r := deploystorage.ToStorageNetworkPlan("", plan.Stack, schema.PackageNames(plan.FocusServer...), ports, plan.IngressFragment)
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

	hints = append(hints, fmt.Sprintf("Inspecting your deployment: %s",
		colors.Ctx(ctx).Highlight.Apply(fmt.Sprintf("ns t kubectl %s get pods", envLabel))))

	for _, hint := range hints {
		fmt.Fprintf(out, "   · %s\n", hint)
	}

	return nil
}

func uploadPlanTo(ctx context.Context, target compute.Computable[oci.RepositoryWithParent], plan *schema.DeployPlan) error {
	any, err := anypb.New(plan)
	if err != nil {
		return err
	}

	messages, err := protos.SerializeOpts{TextProto: true}.Serialize(any)
	if err != nil {
		return err
	}

	var contents memfs.FS
	for ext, data := range messages[0].PerFormat {
		contents.Add(fmt.Sprintf("deployplan.%s", ext), data)
	}

	image := oci.MakeImageFromScratch("deploy plan", oci.MakeLayer("deploy plan contents", compute.Precomputed[fs.FS](&contents, digestfs.Digest)))

	result := oci.PublishImage(target, image)
	resultImageID, err := compute.GetValue(ctx, result.ImageID())
	if err != nil {
		return err
	}

	fmt.Fprintf(console.Stdout(ctx), "Pushed plan to %s\n", resultImageID.RepoAndDigest())
	return nil
}

func deployReason(opts deployOpts) string {
	if opts.manualReason != "" {
		return opts.manualReason
	}

	if buildkite.IsRunningInBuildkite() {
		return buildkite.BuildMessage()
	}

	return ""
}
