// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBuildBinaryCmd() *cobra.Command {
	all := false
	envRef := "dev"
	publishToDocker := false
	keepRepositories := true
	outputPrebuilts := false

	cmd := &cobra.Command{
		Use:   "build-binary",
		Short: "Builds the specified tool binary.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			var locs []fnfs.Location
			if all {
				list, err := workspace.ListSchemas(ctx, root)
				if err != nil {
					return err
				}

				locs = list.Locations
			} else {
				for _, arg := range args {
					_, loc, err := module.PackageAt(ctx, arg)
					if err != nil {
						return err
					}
					locs = append(locs, loc)
				}
			}

			return buildLocations(ctx, root, locs, envRef, keepRepositories, publishToDocker, outputPrebuilts)
		}),
	}

	cmd.Flags().BoolVar(&all, "all", all, "Build all images in the current workspace.")
	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to build for (as defined in the workspace).")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
	cmd.Flags().BoolVar(&publishToDocker, "docker", publishToDocker, "If set to true, don't push to registries, but to local docker.")
	cmd.Flags().BoolVar(&keepRepositories, "keep_repositories", keepRepositories, "If set to true, re-uses the configured repository (as opposed to env-bound repository).")
	cmd.Flags().BoolVar(&outputPrebuilts, "output_prebuilts", outputPrebuilts, "If true, also outputs a prebuilt configuration which can be embedded in your workspace configuration.")

	return cmd
}

func buildLocations(ctx context.Context, root *workspace.Root, list []fnfs.Location, envRef string, keepRepositories, publishToDocker, outputPrebuilts bool) error {
	bid := provision.NewBuildID()

	env, err := provision.RequireEnv(root, envRef)
	if err != nil {
		return err
	}

	pl := workspace.NewPackageLoader(root)

	var pkgs []*workspace.Package
	for _, loc := range list {
		pkg, err := pl.LoadByName(ctx, loc.AsPackageName())
		if err != nil {
			return err
		}

		if pkg.Binary == nil {
			continue
		}

		if pkg.Binary.Repository == "" {
			fmt.Fprintf(console.Stderr(ctx), "Skipping %q, no repository defined.\n", pkg.Binary.PackageName)
			continue
		}

		pkgs = append(pkgs, pkg)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return strings.Compare(pkgs[i].PackageName().String(), pkgs[j].PackageName().String()) < 0
	})

	sealed := pl.Seal()
	boundEnv := env.BindWith(sealed)

	var opts binary.BuildImageOpts
	opts.UsePrebuilts = false
	opts.Platforms = []specs.Platform{docker.HostPlatform()}

	var images []compute.Computable[oci.ImageID]
	for _, pkg := range pkgs {
		bin, err := binary.Plan(ctx, pkg, opts)
		if err != nil {
			return err
		}

		image, err := bin.Image(ctx, env)
		if err != nil {
			return err
		}

		tag, err := binary.MakeTag(ctx, boundEnv, pkg, bid, keepRepositories)
		if err != nil {
			return err
		}

		if publishToDocker {
			images = append(images, docker.PublishImage(tag, image))
		} else {
			images = append(images, oci.PublishResolvable(tag, image))
		}
	}

	res, err := compute.Get(ctx, compute.Collect(tasks.Action("fn.build-binary"), images...))
	if err != nil {
		return err
	}

	for k, r := range res.Value {
		fmt.Fprintf(console.Stdout(ctx), "%s: %s\n", pkgs[k].PackageName(), r.Value)
	}

	if outputPrebuilts {
		ws := &schema.Workspace{}

		for k, r := range res.Value {
			prebuilt := &schema.Workspace_BinaryDigest{
				PackageName: pkgs[k].PackageName().String(),
				Digest:      r.Value.Digest,
			}
			ws.PrebuiltBinary = append(ws.PrebuiltBinary, prebuilt)
		}

		workspace.FormatWorkspace(console.Stdout(ctx), ws)
	}

	return nil
}
