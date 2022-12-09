// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/production"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func Build(ctx context.Context, env cfg.Context, bin GoBinary, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if conf.Workspace() == nil {
		panic(conf)
	}

	return buildLocalImage(ctx, env, conf.Workspace(), bin, conf)
}

func buildLocalImage(ctx context.Context, env cfg.Context, workspace build.Workspace, bin GoBinary, target build.BuildTarget) (compute.Computable[oci.Image], error) {
	sdk, err := golang.MatchSDK(bin.GoVersion, host.HostPlatform())
	if err != nil {
		return nil, err
	}

	layers := []oci.NamedLayer{
		// By depending on workspace.Contents we both get continued updates on changes to the workspace,
		// but also are guaranteed to only be invoked after generation functions run.
		oci.MakeLayer(fmt.Sprintf("go binary layer %s", bin.PackageName), &compilation{
			sdk:       sdk,
			workspace: workspace.Snapshot(bin.GoModulePath, bin.isFocus),
			binary:    bin,
			platform:  *target.TargetPlatform(),
		}),
	}

	if bin.BinaryOnly {
		return oci.MakeImageFromScratch(fmt.Sprintf("Go binary %s", bin.PackageName), layers...).Image(), nil
	}

	base, err := baseImage(ctx, env, target)
	if err != nil {
		return nil, err
	}

	return compute.Named(tasks.Action("go.make-binary-image").Arg("binary", bin),
		oci.MakeImage(fmt.Sprintf("Go binary %s", bin.PackageName), base, layers...).Image()), nil
}

func baseImage(ctx context.Context, env cfg.Context, target build.BuildTarget) (oci.NamedImage, error) {
	// We use a different base for development because most Kubernetes installations don't
	// yet support ephemeral containers, which would allow us to side-load into the same
	// namespace as the running server, for debugging. So we instead add a base with some
	// utilities so we can exec into the server. There's a tension here on reproducibility,
	// as the server could inadvertidely depend on these utilities. But we only do this for
	// DEVELOPMENT, not for TESTING.
	if env.Environment().Purpose == schema.Environment_DEVELOPMENT {
		return production.DevelopmentImage(ctx, production.Alpine, buildkit.DeferClient(env.Configuration(), target.TargetPlatform()), target)
	}

	return production.ServerImage(production.Distroless, *target.TargetPlatform())
}

func platformToEnv(platform specs.Platform, cgo int) []string {
	return []string{fmt.Sprintf("CGO_ENABLED=%d", cgo), "GOOS=" + platform.OS, "GOARCH=" + platform.Architecture}
}

func compile(ctx context.Context, sdk golang.LocalSDK, absWorkspace string, targetDir string, bin GoBinary, platform specs.Platform) error {
	env := platformToEnv(platform, 0)

	if platform.Architecture == "arm" {
		v, err := goarm(platform)
		if err != nil {
			return err
		}
		env = append(env, v)
	}

	modulePath := filepath.Join(absWorkspace, bin.GoModulePath)
	out := filepath.Join(targetDir, bin.BinaryName)
	pkg := makePkg(bin.SourcePath)

	var cmd localexec.Command
	cmd.Label = "go build"
	cmd.Command = golang.GoBin(sdk)
	cmd.Args = append(goBuildArgs(sdk.Version), "-o="+out, pkg)
	cmd.AdditionalEnv = append(env, makeGoEnv(sdk)...)
	cmd.Dir = modulePath
	return cmd.Run(ctx)
}

func makePkg(srcPath string) string {
	if srcPath == "" || srcPath == "." {
		return "./"
	}

	return "./" + srcPath
}

func goarm(platform specs.Platform) (string, error) {
	if platform.Architecture != "arm" {
		return "", fmt.Errorf("not arm: %v", platform.Architecture)
	}
	v := platform.Variant
	if len(v) != 2 {
		return "", fmt.Errorf("unexpected varient: %v", v)
	}
	if v[0] != 'v' || !('0' <= v[1] && v[1] <= '9') {
		return "", fmt.Errorf("unexpected varient: %v", v)
	}
	return string(v[1]), nil
}

type compilation struct {
	sdk       compute.Computable[golang.LocalSDK]
	workspace compute.Computable[wscontents.Versioned] // We depend on `workspace` so we trigger a re-build on workspace changes.
	binary    GoBinary
	platform  specs.Platform

	compute.LocalScoped[fs.FS]
}

func (c *compilation) Action() *tasks.ActionEvent {
	return tasks.Action("go.build.binary").
		Arg("binary", c.binary.BinaryName).
		Arg("module_path", c.binary.GoModulePath).
		Arg("source_path", c.binary.SourcePath).
		Arg("platform", platform.FormatPlatform(c.platform))
}

func (c *compilation) Inputs() *compute.In {
	in := compute.Inputs().
		JSON("binary", c.binary).
		JSON("platform", c.platform).
		Computable("workspace", c.workspace).
		Computable("sdk", c.sdk)
	if !c.binary.UnsafeCacheable {
		in = in.Indigestible("localfs", nil)
	}
	return in
}

func (c *compilation) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	w := compute.MustGetDepValue(deps, c.workspace, "workspace")
	sdk := compute.MustGetDepValue(deps, c.sdk, "sdk")

	targetDir, err := dirs.CreateUserTempDir("go", "build")
	if err != nil {
		return nil, err
	}

	if err := compile(ctx, sdk, w.Abs(), targetDir, c.binary, c.platform); err != nil {
		return nil, err
	}

	result := fnfs.Local(targetDir)

	// Only initiate a cleanup after we're done compiling.
	compute.On(ctx).Cleanup(tasks.Action("go.build.cleanup"), func(ctx context.Context) error {
		if err := os.RemoveAll(targetDir); err != nil {
			fmt.Fprintln(console.Warnings(ctx), "failed to cleanup target dir", err)
		}
		return nil // Never fail.
	})

	return result, nil
}
