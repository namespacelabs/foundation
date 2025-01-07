// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewBuildBinaryCmd() *cobra.Command {
	var (
		baseRepository []string
		userTag        string
		buildOpts      buildOpts
		env            cfg.Context
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
			flags.StringSliceVar(&baseRepository, "base_repository", baseRepository, "If set, overrides the registry we'll upload the images to.")
			flags.BoolVar(&buildOpts.outputPrebuilts, "output_prebuilts", false, "If true, also outputs a prebuilt configuration which can be embedded in your workspace configuration.")
			flags.StringVar(&buildOpts.outputPath, "output_to", "", "If set, a list of all binaries is emitted to the specified file.")
			flags.StringVar(&userTag, "tag", "", "Which tag to attach to each of the built images.")
		}).
		With(
			fncobra.ParseEnv(&env),
			fncobra.ParseLocations(&cmdLocs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true, SupportPackageRef: true})).
		Do(func(ctx context.Context) error {
			registry, err := registry.GetRegistry(ctx, env)
			if err != nil {
				return err
			}

			return buildLocations(ctx, env, registry, userTag, cmdLocs, baseRepository, buildOpts)
		})
}

type buildOpts struct {
	publishToDocker bool
	outputPrebuilts bool
	outputPath      string
}

const orchTool = "namespacelabs.dev/foundation/orchestration/server/tool"

func buildLocations(ctx context.Context, env cfg.Context, reg registry.Manager, userTag string, locs fncobra.Locations, baseRepository []string, opts buildOpts) error {
	if opts.outputPrebuilts && len(baseRepository) == 0 {
		return fnerrors.New("at least one repository has to be set when updating prebuilts")
	}

	pl := parsing.NewPackageLoader(env)

	var pkglist schema.PackageList
	for _, ref := range locs.Refs {
		pkglist.Add(ref.AsPackageName())
	}
	for _, loc := range locs.Locations {
		pkglist.Add(loc.AsPackageName())
	}

	var pkgs []*pkggraph.Package
	for _, loc := range pkglist.PackageNames() {
		if !locs.UserSpecified && loc.Equals(orchTool) {
			// Skip the orchestration server tool by default.
			// TODO scale this if we see a need.
			continue
		}

		pkg, err := pl.LoadByName(ctx, loc)
		if err != nil {
			return err
		}

		// TODO: allow to choose what binary to build within a package.
		if len(pkg.Binaries) > 0 {
			pkgs = append(pkgs, pkg)
		} else if locs.UserSpecified {
			return fnerrors.NewWithLocation(loc, "no binary found in package")
		}
	}

	sort.Slice(pkgs, func(i, j int) bool {
		return strings.Compare(pkgs[i].PackageName().String(), pkgs[j].PackageName().String()) < 0
	})

	sealedCtx := pkggraph.MakeSealedContext(env, pl.Seal())

	var imgOpts binary.BuildImageOpts
	imgOpts.UsePrebuilts = false
	imgOpts.Platforms = []specs.Platform{docker.HostPlatform()}

	var images []compute.Computable[Binary]
	for _, pkg := range pkgs {
		var resolvables []compute.Computable[oci.ResolvableImage]

		// TODO: allow to choose what binary to build within a package.
		for _, b := range pkg.Binaries {
			bin, err := binary.Plan(ctx, pkg, b.Name, sealedCtx, assets.AvailableBuildAssets{}, imgOpts)
			if err != nil {
				return err
			}

			image, err := bin.Image(ctx, sealedCtx)
			if err != nil {
				return err
			}

			resolvables = append(resolvables, image)
		}

		for _, image := range resolvables {
			var repositories []compute.Computable[oci.RepositoryWithParent]
			for _, bp := range baseRepository {
				repositories = append(repositories, registry.StaticRepository(nil, filepath.Join(bp, pkg.PackageName().String()), oci.RegistryAccess{}))
			}

			if len(repositories) == 0 {
				repositories = append(repositories, reg.AllocateName(pkg.PackageName().String(), userTag))
			}

			for _, repository := range repositories {
				var img compute.Computable[oci.ImageID]
				if opts.publishToDocker {
					img = docker.PublishImage(repository, image)
				} else {
					img = oci.PublishResolvable(repository, image, nil)
				}

				images = append(images, fromImage(pkg.PackageName(), img))
			}
		}
	}

	res, err := compute.Get(ctx, compute.Collect(tasks.Action("fn.build-binary"), images...))
	if err != nil {
		return err
	}

	if opts.outputPath != "" {
		out := &bytes.Buffer{}
		for _, r := range res.Value {
			fmt.Fprintf(out, "%s\n", r.Value.img)
		}
		if err := os.WriteFile(opts.outputPath, out.Bytes(), 0644); err != nil {
			return fnerrors.New("failed to write %q: %w", opts.outputPath, err)
		}
	}

	if len(res.Value) == 1 {
		fmt.Fprintf(console.Stdout(ctx), "%s\n", res.Value[0].Value.img)
	} else {
		for _, r := range res.Value {
			fmt.Fprintf(console.Stdout(ctx), "%s: %s\n", r.Value.pkg, r.Value.img)
		}
	}

	if opts.outputPrebuilts && len(res.Value) > 0 {
		var digestFields []interface{}

		for _, r := range res.Value {
			digestFields = append(digestFields, &ast.Field{
				Label: ast.NewString(r.Value.pkg.String()),
				Value: ast.NewString(r.Value.img.Digest),
			})
		}

		p := ast.NewStruct(&ast.Field{
			Label: ast.NewIdent("prebuilts"),
			Value: ast.NewStruct(&ast.Field{
				Label: ast.NewIdent("digest"),
				Value: ast.NewStruct(digestFields...),
			}, &ast.Field{
				Label: ast.NewIdent("baseRepository"),
				Value: ast.NewString(baseRepository[0]),
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

type Binary struct {
	pkg schema.PackageName // TODO sufficient key? What about multiple bins in one package?
	img oci.ImageID
}

func fromImage(pkg schema.PackageName, img compute.Computable[oci.ImageID]) compute.Computable[Binary] {
	return &transformImg{pkg: pkg, img: img}
}

type transformImg struct {
	pkg schema.PackageName
	img compute.Computable[oci.ImageID]

	compute.LocalScoped[Binary]
}

func (i *transformImg) Action() *tasks.ActionEvent {
	return tasks.Action("transform.img")
}

func (i *transformImg) Inputs() *compute.In {
	return compute.Inputs().Stringer("pkg", i.pkg).Computable("img", i.img)
}

func (i *transformImg) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (i *transformImg) Compute(ctx context.Context, deps compute.Resolved) (Binary, error) {
	img := compute.MustGetDepValue(deps, i.img, "img")

	return Binary{pkg: i.pkg, img: img}, nil
}
