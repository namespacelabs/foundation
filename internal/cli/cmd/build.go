// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBuildCmd() *cobra.Command {
	var (
		explain      = false
		continuously = false
	)

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build one, or more servers.",
		Long:  "Build one, or more servers.\nAutomatically invoked with `deploy`.",
		Args:  cobra.ArbitraryArgs,
	}

	cmd.Flags().BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
	cmd.Flags().BoolVarP(&continuously, "continuously", "c", continuously, "If set to true, builds continuously, listening to changes to the workspace.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		serverLocs, specified, err := allServersOrFromArgs(ctx, env, args)
		if err != nil {
			return err
		}

		_, servers, err := loadServers(ctx, env, serverLocs, specified)
		if err != nil {
			return err
		}

		var opts deploy.Opts
		opts.BaseServerPort = 10000

		_, images, err := deploy.ComputeStackAndImages(ctx, env, servers, opts)
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

func outputResults(ctx context.Context, results []compute.ResultWithTimestamp[oci.ImageID]) {
	out := console.TypedOutput(ctx, "build", console.CatOutputUs)

	sort.Slice(results, func(i, j int) bool {
		if results[i].Timestamp.Equal(results[j].Timestamp) {
			imgI := results[i].Value
			imgJ := results[j].Value

			return strings.Compare(imgI.String(), imgJ.String()) < 0
		} else {
			return results[i].Timestamp.Before(results[j].Timestamp)
		}
	})

	fmt.Fprintf(out, "Got %d images:\n\n", len(results))

	for k, it := range results {
		img := it.Value

		if k > 0 && !it.Timestamp.IsZero() && results[k-1].Timestamp.IsZero() {
			fmt.Fprintln(out)
		}

		fmt.Fprint(out, "  ")
		if it.Timestamp.IsZero() {
			fmt.Fprint(out, colors.Faded("prebuilt "))
		}

		fmt.Fprintln(out, img)
		if !it.Timestamp.IsZero() {
			fmt.Fprintln(out, colors.Faded(fmt.Sprintf("     built %v", it.Timestamp)))
		}
	}
}

type continuousBuild struct {
	allImages compute.Computable[[]compute.ResultWithTimestamp[oci.ImageID]]
}

func (c continuousBuild) Inputs() *compute.In {
	return compute.Inputs().Computable("all-images", c.allImages)
}
func (c continuousBuild) Cleanup(context.Context) error { return nil }
func (c continuousBuild) Updated(ctx context.Context, deps compute.Resolved) error {
	outputResults(ctx, compute.MustGetDepValue(deps, c.allImages, "all-images"))
	return nil
}

func requireEnv(ctx context.Context, envRef string) (provision.Env, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return provision.Env{}, err
	}

	return provision.RequireEnv(root, envRef)
}

func allServersOrFromArgs(ctx context.Context, env provision.Env, args []string) ([]fnfs.Location, bool, error) {
	if len(args) == 0 {
		schemaList, err := workspace.ListSchemas(ctx, env.Root())
		if err != nil {
			return nil, false, err
		}

		return schemaList.Locations, false, nil
	}

	var locations []fnfs.Location
	for _, arg := range args {
		// XXX RelPackage should probably validate that it's a valid path (e.g. doesn't escape module).
		loc := env.Root().RelPackage(arg)
		locations = append(locations, loc)
	}

	return locations, true, nil
}

func loadServers(ctx context.Context, env provision.Env, locations []fnfs.Location, specified bool) (workspace.SealedPackages, []provision.Server, error) {
	loader := workspace.NewPackageLoader(env.Root())

	var servers []provision.Server
	for _, loc := range locations {
		if err := tasks.Action("package.load-server").Scope(loc.AsPackageName()).Run(ctx, func(ctx context.Context) error {
			pp, err := loader.LoadByName(ctx, loc.AsPackageName())
			if err != nil {
				return fnerrors.Wrap(loc, err)
			}

			if pp.Server == nil {
				if specified {
					return fnerrors.UserError(loc, "expected a server")
				}

				return nil
			}

			srv, err := env.RequireServerWith(ctx, loader, loc.AsPackageName())
			if err != nil {
				return fnerrors.Wrap(loc, err)
			}

			// If the user doesn't explicitly specify this server should be loaded, don't load it, if it's tagged as being testonly.
			if !specified && srv.Package.Server.Testonly {
				return nil
			}

			servers = append(servers, srv)
			return nil
		}); err != nil {
			return nil, nil, err
		}
	}

	return loader.Seal(), servers, nil
}
