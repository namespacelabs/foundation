// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/bazelbuild/bazelisk/core"
	"github.com/bazelbuild/bazelisk/repositories"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func buildBazelImage(ctx context.Context, env pkggraph.SealedContext, workspace build.Workspace, bin GoBinary, target build.BuildTarget, bazelrc string) (compute.Computable[oci.Image], error) {
	if workspace == nil {
		return nil, fnerrors.InternalError("bazel: workspace is missing")
	}
	if target.TargetPlatform() == nil {
		return nil, fnerrors.InternalError("bazel: target platform is missing")
	}
	if bazelrc == "" {
		return nil, fnerrors.InternalError("bazel: bazelrc is missing")
	}

	comp := &bazelCompilation{
		binary:       bin,
		platform:     *target.TargetPlatform(),
		workspaceAbs: workspace.Abs(),
		bazelrc:      bazelrc,
		trigger:      workspace.ChangeTrigger(".", nil),
	}

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
	workspaceAbs string
	bazelrc      string
	trigger      compute.Computable[any]
	binary       GoBinary
	platform     specs.Platform

	compute.LocalScoped[fs.FS]
}

func (c *bazelCompilation) Action() *tasks.ActionEvent {
	return tasks.Action("go.build.binary.bazel").Arg("binary", c.binary.BinaryName).Arg("target", c.binary.BazelPackagePath).Arg("platform", platform.FormatPlatform(c.platform))
}

func (c *bazelCompilation) Inputs() *compute.In {
	in := compute.Inputs().JSON("binary", c.binary).JSON("platform", c.platform).Str("bazelrc", c.bazelrc)
	if c.trigger != nil {
		in = in.Computable("trigger", c.trigger)
	}
	return in
}

func (c *bazelCompilation) Compute(ctx context.Context, _ compute.Resolved) (fs.FS, error) {
	target, err := bazelTarget(c.binary)
	if err != nil {
		return nil, err
	}
	goPlatform, err := rulesGoPlatform(c.platform)
	if err != nil {
		return nil, err
	}

	config := core.MakeDefaultConfig()
	gcs := &repositories.GCSRepo{}
	github := repositories.CreateGitHubRepo(config.Get("BAZELISK_GITHUB_TOKEN"))
	repos := core.CreateRepositories(gcs, github, gcs, gcs, true)
	installation, err := core.GetBazelInstallation(repos, config)
	if err != nil {
		return nil, fnerrors.Newf("bazel: failed to install Bazel: %w", err)
	}

	startup := []string{"--bazelrc=" + c.bazelrc}
	buildArgs := append(startup, "build", "--platforms="+goPlatform, "--remote_download_outputs=all", target)
	if err := runBazel(ctx, installation.Path, c.workspaceAbs, buildArgs...); err != nil {
		return nil, err
	}

	var stdout bytes.Buffer
	cqueryArgs := append(startup, "cquery", "--output=files", "--platforms="+goPlatform, target)
	if err := runBazelWithOutput(ctx, installation.Path, c.workspaceAbs, &stdout, cqueryArgs...); err != nil {
		return nil, err
	}
	outputs := strings.Fields(stdout.String())
	if len(outputs) != 1 {
		return nil, fnerrors.Newf("bazel: target %s produced %d files, expected one: %q", target, len(outputs), stdout.String())
	}

	source := outputs[0]
	if !filepath.IsAbs(source) {
		source = filepath.Join(c.workspaceAbs, source)
	}
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

func runBazel(ctx context.Context, binary, dir string, args ...string) error {
	return runBazelWithOutput(ctx, binary, dir, console.Output(ctx, "bazel"), args...)
}

func runBazelWithOutput(ctx context.Context, binary, dir string, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = console.Output(ctx, "bazel")
	if err := cmd.Run(); err != nil {
		return fnerrors.Newf("bazel: command failed: %w", err)
	}
	return nil
}
