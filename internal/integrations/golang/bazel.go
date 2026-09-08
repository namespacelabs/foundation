// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	buildbazel "namespacelabs.dev/foundation/internal/build/bazel"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func buildBazelImage(ctx context.Context, env pkggraph.SealedContext, workspace build.Workspace, bin GoBinary, target build.BuildTarget, builder *buildbazel.Builder) (compute.Computable[oci.Image], error) {
	if workspace == nil {
		return nil, fnerrors.InternalError("bazel: workspace is missing")
	}
	if target.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("bazel: target platform is missing")
	}
	if builder == nil {
		return nil, fnerrors.InternalError("bazel: builder is missing")
	}

	label, err := bazelTarget(bin)
	if err != nil {
		return nil, err
	}
	goPlatform, err := rulesGoPlatform(*target.TargetPlatform())
	if err != nil {
		return nil, err
	}
	output, err := builder.AddTarget(ctx, buildbazel.Target{
		WorkspaceAbs: workspace.Abs(),
		Label:        label,
		BuildArgs:    []string{"--platforms=" + goPlatform},
	})
	if err != nil {
		return nil, err
	}
	comp := &bazelCompilation{binary: bin, target: label, output: output}

	layers := []oci.NamedLayer{oci.MakeLayer(fmt.Sprintf("go binary layer %s", bin.PackageName), comp)}
	if bin.BinaryOnly {
		return oci.MakeImageFromScratch(fmt.Sprintf("Go binary %s", bin.PackageName), layers...).Image(), nil
	}

	base, err := baseImage(ctx, env, target)
	if err != nil {
		return nil, err
	}
	return compute.Named(tasks.Action("go.make-binary-image").Arg("binary", bin), oci.MakeImage(fmt.Sprintf("Go binary %s", bin.PackageName), base, layers...).Image()), nil
}

func bazelTarget(bin GoBinary) (string, error) {
	pkg := filepath.ToSlash(filepath.Clean(bin.BazelPackagePath))
	if pkg == "." || pkg == "" {
		name := path.Base(bin.GoModule)
		if name == "." || name == "" {
			return "", fnerrors.Newf("bazel: unable to derive a target for Go module %q", bin.GoModule)
		}
		return "//:" + name, nil
	}
	if strings.HasPrefix(pkg, "../") {
		return "", fnerrors.Newf("bazel: Go package path %q is outside the Bazel workspace", bin.BazelPackagePath)
	}
	return fmt.Sprintf("//%s:%s", pkg, path.Base(pkg)), nil
}

func rulesGoPlatform(p specs.Platform) (string, error) {
	if p.OS == "" || p.Architecture == "" || p.Variant != "" {
		return "", fnerrors.Newf("bazel: unsupported target platform %q", platform.FormatPlatform(p))
	}
	return fmt.Sprintf("@rules_go//go/toolchain:%s_%s", p.OS, p.Architecture), nil
}

type bazelCompilation struct {
	binary GoBinary
	target string
	output compute.Computable[string]

	compute.LocalScoped[fs.FS]
}

func (c *bazelCompilation) Action() *tasks.ActionEvent {
	return tasks.Action("go.build.binary.bazel").Arg("binary", c.binary.BinaryName).Arg("target", c.target)
}

func (c *bazelCompilation) Inputs() *compute.In {
	return compute.Inputs().JSON("binary", c.binary).Computable("bazel", c.output)
}

func (c *bazelCompilation) Compute(ctx context.Context, deps compute.Resolved) (fs.FS, error) {
	source := compute.MustGetDepValue(deps, c.output, "bazel")

	targetDir, err := dirs.CreateUserTempDir("bazel", "build")
	if err != nil {
		return nil, err
	}
	destination := filepath.Join(targetDir, c.binary.BinaryName)
	if err := copyExecutable(source, destination); err != nil {
		return nil, fnerrors.Newf("bazel: failed to copy output: %w", err)
	}

	compute.On(ctx).Cleanup(tasks.Action("go.build.cleanup"), func(context.Context) error {
		if err := os.RemoveAll(targetDir); err != nil {
			fmt.Fprintln(console.Warnings(ctx), "failed to cleanup target dir", err)
		}
		return nil
	})
	return fnfs.Local(targetDir), nil
}

func copyExecutable(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
