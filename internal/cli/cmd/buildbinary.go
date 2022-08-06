// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"

	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
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
		baseRepository string
		buildOpts      buildOpts
		env            provision.Env
		cmdLocs        fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "build-binary [path/to/package]...",
			Short: "Builds the specified tool binary.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.Var(build.BuildPlatformsVar{}, "build_platforms", "Allows the runtime to be instructed to build for a different set of platforms; by default we only build for the development host.")
			flags.BoolVar(&buildOpts.publishToDocker, "docker", false, "If set to true, don't push to registries, but to local docker.")
			flags.StringVar(&baseRepository, "base_repository", baseRepository, "If set, overrides the registry we'll upload the images to.")
			flags.BoolVar(&buildOpts.outputPrebuilts, "output_prebuilts", false, "If true, also outputs a prebuilt configuration which can be embedded in your workspace configuration.")
			flags.StringVar(&buildOpts.outputPath, "output_to", "", "If set, a list of all binaries is emitted to the specified file.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&cmdLocs, &fncobra.ParseLocationsOpts{DefaultToAllWhenEmpty: true})).
		Do(func(ctx context.Context) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			return buildLocations(ctx, root, cmdLocs.Locs, env, baseRepository, buildOpts)
		})
}

type buildOpts struct {
	publishToDocker bool
	outputPrebuilts bool
	outputPath      string
}

func buildLocations(ctx context.Context, root *workspace.Root, list []fnfs.Location, env provision.Env, baseRepository string, opts buildOpts) error {
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

	var imgOpts binary.BuildImageOpts
	imgOpts.UsePrebuilts = false
	imgOpts.Platforms = []specs.Platform{docker.HostPlatform()}

	var images []compute.Computable[oci.ImageID]
	for _, pkg := range pkgs {
		bin, err := binary.Plan(ctx, pkg, imgOpts)
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
			}, nil)
		} else {
			tag, err = registry.AllocateName(ctx, env, pkg.PackageName())
			if err != nil {
				return err
			}
		}

		if opts.publishToDocker {
			images = append(images, docker.PublishImage(tag, image))
		} else {
			images = append(images, oci.PublishResolvable(tag, image))
		}
	}

	res, err := compute.Get(ctx, compute.Collect(tasks.Action("fn.build-binary"), images...))
	if err != nil {
		return err
	}

	if opts.outputPath != "" {
		out := &bytes.Buffer{}
		for _, r := range res.Value {
			fmt.Fprintf(out, "%s\n", r.Value)
		}
		if err := ioutil.WriteFile(opts.outputPath, out.Bytes(), 0644); err != nil {
			return fnerrors.New("failed to write %q: %w", opts.outputPath, err)
		}
	}

	if len(res.Value) == 1 {
		fmt.Fprintf(console.Stdout(ctx), "%s\n", res.Value[0].Value)
	} else {
		for k, r := range res.Value {
			fmt.Fprintf(console.Stdout(ctx), "%s: %s\n", pkgs[k].PackageName(), r.Value)
		}
	}

	if opts.outputPrebuilts && len(res.Value) > 0 {
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
