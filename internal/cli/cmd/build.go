// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBuildCmd() *cobra.Command {
	var (
		explain      = false
		continuously = false
		env          provision.Env
		locs         fncobra.Locations
		servers      fncobra.Servers
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "build",
			Short: "Build one, or more servers.",
			Long:  "Build one, or more servers.\nAutomatically invoked with `deploy`.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
			flags.BoolVarP(&continuously, "continuously", "c", continuously, "If set to true, builds continuously, listening to changes to the workspace.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{DefaultToAllWhenEmpty: true}),
			fncobra.ParseServers(&servers, &env, &locs)).
		DoWithArgs(func(ctx context.Context, args []string) error {
			_, images, err := deploy.ComputeStackAndImages(ctx, env, servers.Servers)
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
			return nil
		})
}

func outputResults(ctx context.Context, results []compute.ResultWithTimestamp[deploy.ResolvedServerImages]) {
	out := console.TypedOutput(ctx, "build", console.CatOutputUs)

	slices.SortFunc(results, func(a, b compute.ResultWithTimestamp[deploy.ResolvedServerImages]) bool {
		return strings.Compare(a.Value.Package.String(), b.Value.Package.String()) < 0
	})

	style := colors.Ctx(ctx)
	for k, it := range results {
		if k > 0 {
			fmt.Fprintln(out)
		}

		resolved := it.Value

		fmt.Fprintf(out, "  %s\n", resolved.Package)

		fmt.Fprintf(out, "    %s ", style.Header.Apply("Binary:"))
		if resolved.PrebuiltBinary {
			fmt.Fprint(out, style.LessRelevant.Apply("prebuilt "))
		}

		fmt.Fprintf(out, "%s\n", resolved.Binary)

		if resolved.Config.String() != "" {
			fmt.Fprintf(out, "    %s %s\n", style.Header.Apply("Config:"), resolved.Config)
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
