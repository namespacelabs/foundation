// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBuildCmd() *cobra.Command {
	var (
		explain                = false
		continuously           = false
		prebuiltBaseRepository string
		env                    planning.Context
		locs                   fncobra.Locations
		servers                fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "build [path/to/server]...",
			Short: "Build one, or more servers.",
			Long:  "Build one, or more servers.\nAutomatically invoked with `deploy`.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
			flags.BoolVarP(&continuously, "continuously", "c", continuously, "If set to true, builds continuously, listening to changes to the workspace.")
			flags.StringVar(&prebuiltBaseRepository, "base_repository", "", "If set, also uploads the server binary build to the target prebuilt repository.")

			// "base_repository" is used to keep consistency with `build-binary`.
			_ = flags.MarkHidden("base_repository")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		Do(func(ctx context.Context) error {
			if prebuiltBaseRepository != "" {
				if explain || continuously {
					return fnerrors.BadInputError("base_repository is not compatible with explain or continuously")
				}
			}

			cluster, err := runtime.PlannerFor(ctx, env)
			if err != nil {
				return err
			}

			_, images, err := deploy.ComputeStackAndImages(ctx, env, cluster, servers.Servers)
			if err != nil {
				return err
			}

			buildAll := compute.Collect(tasks.Action("build.all-images"), images...)

			if explain {
				return compute.Explain(ctx, console.Stdout(ctx), buildAll)
			}

			if continuously {
				console.SetIdleLabel(ctx, "waiting for workspace changes")
				return compute.Continuously(ctx, continuousBuild{allImages: buildAll}, nil)
			}

			res, err := compute.GetValue(ctx, buildAll)
			if err != nil {
				return err
			}

			outputResults(ctx, res)

			if prebuiltBaseRepository != "" {
				return writePrebuilts(ctx, prebuiltBaseRepository, res)
			}

			return nil
		})
}

func outputResults(ctx context.Context, results []compute.ResultWithTimestamp[deploy.ResolvedServerImages]) {
	out := console.TypedOutput(ctx, "build", console.CatOutputUs)

	slices.SortFunc(results, func(a, b compute.ResultWithTimestamp[deploy.ResolvedServerImages]) bool {
		return a.Value.PackageRef.Compare(b.Value.PackageRef) < 0
	})

	style := colors.Ctx(ctx)
	for k, it := range results {
		if k > 0 {
			fmt.Fprintln(out)
		}

		resolved := it.Value

		fmt.Fprintf(out, "  %s\n", resolved.PackageRef.Canonical())

		fmt.Fprintf(out, "    %s ", style.Header.Apply("Binary:"))
		if resolved.PrebuiltBinary {
			fmt.Fprint(out, style.LessRelevant.Apply("prebuilt "))
		}

		fmt.Fprintf(out, "%s\n", resolved.Binary)

		if resolved.Config.String() != "" {
			fmt.Fprintf(out, "    %s %s\n", style.Header.Apply("Config:"), resolved.Config)
		}

		for _, sidecar := range resolved.Sidecars {
			fmt.Fprintf(out, "    %s %s %s\n", style.Header.Apply("Sidecar:"), sidecar.PackageRef.Canonical(), sidecar.Binary)
		}
	}
}

type continuousBuild struct {
	allImages compute.Computable[[]compute.ResultWithTimestamp[deploy.ResolvedServerImages]]
}

func (c continuousBuild) Inputs() *compute.In {
	return compute.Inputs().Computable("all-images", c.allImages)
}
func (c continuousBuild) Cleanup(context.Context) error { return nil }
func (c continuousBuild) Updated(ctx context.Context, deps compute.Resolved) error {
	outputResults(ctx, compute.MustGetDepValue(deps, c.allImages, "all-images"))
	return nil
}

func writePrebuilts(ctx context.Context, baseRepository string, results []compute.ResultWithTimestamp[deploy.ResolvedServerImages]) error {
	out := console.TypedOutput(ctx, "prebuilts", console.CatOutputUs)

	var outputs []compute.Computable[oci.ImageID]
	var packages []*schema.PackageRef
	for _, res := range results {
		v := res.Value

		target := registry.StaticName(nil, oci.ImageID{
			Repository: filepath.Join(baseRepository, v.PackageRef.PackageName),
		}, nil)

		if v.BinaryImage != nil {
			img := oci.PublishResolvable(target, v.BinaryImage)
			outputs = append(outputs, img)
			packages = append(packages, v.PackageRef)
		}
	}

	pushAll := compute.Collect(tasks.Action("build.push-images"), outputs...)
	pushResults, err := compute.GetValue(ctx, pushAll)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\nPrebuilts:\n\n")
	for k, pushed := range pushResults {
		fmt.Fprintf(out, "%s:\n  %s\n", packages[k].Canonical(), pushed.Value.RepoAndDigest())
	}

	return nil
}
