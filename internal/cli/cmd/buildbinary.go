// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"
	"fmt"

	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewBuildBinaryCmd() *cobra.Command {
	var (
		all             = false
		envRef          = "dev"
		publishToDocker = false
		outputPrebuilts = false
		baseRepository  string
	)

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

			return buildLocations(ctx, root, locs, envRef, baseRepository, publishToDocker, outputPrebuilts)
		}),
	}

	cmd.Flags().BoolVar(&all, "all", all, "Build all images in the current workspace.")
	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to build for (as defined in the workspace).")
	cmd.Flags().Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
	cmd.Flags().BoolVar(&publishToDocker, "docker", publishToDocker, "If set to true, don't push to registries, but to local docker.")
	cmd.Flags().StringVar(&baseRepository, "base_repository", baseRepository, "If set, overrides the registry we'll upload the images to.")
	cmd.Flags().BoolVar(&outputPrebuilts, "output_prebuilts", outputPrebuilts, "If true, also outputs a prebuilt configuration which can be embedded in your workspace configuration.")

	return cmd
}

func buildLocations(ctx context.Context, root *workspace.Root, list []fnfs.Location, envRef, baseRepository string, publishToDocker, outputPrebuilts bool) error {
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

		pkgs = append(pkgs, pkg)
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return strings.Compare(pkgs[i].PackageName().String(), pkgs[j].PackageName().String()) < 0
	})

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

		var tag compute.Computable[oci.AllocatedName]
		if baseRepository != "" {
			tag = registry.StaticName(nil, oci.ImageID{
				Repository: filepath.Join(baseRepository, pkg.PackageName().String()),
				Tag:        bid.String(),
			})
		} else {
			tag, err = registry.AllocateName(ctx, env, pkg.PackageName(), bid)
			if err != nil {
				return err
			}
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

	if len(res.Value) == 1 {
		fmt.Fprintf(console.Stdout(ctx), "%s\n", res.Value[0].Value)
	} else {
		for k, r := range res.Value {
			fmt.Fprintf(console.Stdout(ctx), "%s: %s\n", pkgs[k].PackageName(), r.Value)
		}
	}

	if outputPrebuilts && len(res.Value) > 0 {
		var digestFields []interface{}

		for k, pkg := range pkgs {
			digestFields = append(digestFields, &ast.Field{
				Label: ast.NewString(pkg.PackageName().String()),
				Value: ast.NewString(res.Value[k].Value.Digest),
			})
		}

		p := ast.NewStruct(&ast.Field{
			Label: ast.NewIdent("prebuilts"),
			Value: ast.NewStruct(&ast.Field{
				Label: ast.NewIdent("digest"),
				Value: ast.NewStruct(digestFields...),
			}, &ast.Field{
				Label: ast.NewIdent("baseRepository"),
				Value: ast.NewString(baseRepository),
			}),
		})

		formatted, err := format.Node(p)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", formatted)
		return nil
	}

	return nil
}
