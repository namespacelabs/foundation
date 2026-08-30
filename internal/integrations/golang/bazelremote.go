// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/command"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/outerr"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/integrations/bazelremote"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func buildBazelRemoteImage(ctx context.Context, env pkggraph.SealedContext, bin GoBinary, conf build.Configuration) (compute.Computable[oci.Image], error) {
	if conf.Workspace() == nil {
		panic(conf)
	}

	// REAPI actions run on linux/amd64 workers even when Go cross-compiles the requested target.
	sdk, err := golang.SDKReference(bin.GoVersion, specs.Platform{OS: "linux", Architecture: "amd64"})
	if err != nil {
		return nil, err
	}

	comp := &bazelRemoteCompilation{
		binary:       bin,
		platform:     *conf.TargetPlatform(),
		workspaceAbs: conf.Workspace().Abs(),
		trigger:      conf.Workspace().ChangeTrigger(bin.GoWorkspacePath, nil),
		sdkVersion:   sdk.Version,
		sdkURL:       sdk.Ref.URL,
		sdkDigest:    sdk.Ref.Digest.Hex,
	}
	if bin.UnsafeCacheable || conf.Workspace().IsExternal() {
		comp.localfs = memfs.DeferSnapshot(conf.Workspace().ReadOnlyFS(bin.GoWorkspacePath), memfs.SnapshotOpts{})
	}

	layers := []oci.NamedLayer{oci.MakeLayer(fmt.Sprintf("go binary layer %s", bin.PackageName), comp)}
	if bin.BinaryOnly {
		return oci.MakeImageFromScratch(fmt.Sprintf("Go binary %s", bin.PackageName), layers...).Image(), nil
	}

	base, err := baseImage(ctx, env, conf)
	if err != nil {
		return nil, err
	}
	return compute.Named(tasks.Action("go.make-binary-image").Arg("binary", bin), oci.MakeImage(fmt.Sprintf("Go binary %s", bin.PackageName), base, layers...).Image()), nil
}

type bazelRemoteCompilation struct {
	workspaceAbs string
	trigger      compute.Computable[any]
	localfs      compute.Computable[fs.FS]
	binary       GoBinary
	platform     specs.Platform
	sdkVersion   string
	sdkURL       string
	sdkDigest    string

	compute.LocalScoped[fs.FS]
}

func (c *bazelRemoteCompilation) Action() *tasks.ActionEvent {
	return tasks.Action("go.build.binary.remote").
		Arg("binary", c.binary.BinaryName).
		Arg("workspace_path", c.binary.GoWorkspacePath).
		Arg("source_path", c.binary.SourcePath).
		Arg("platform", platform.FormatPlatform(c.platform))
}

func (c *bazelRemoteCompilation) Inputs() *compute.In {
	in := compute.Inputs().
		JSON("binary", c.binary).
		JSON("platform", c.platform).
		Str("sdk_version", c.sdkVersion).
		Str("sdk_url", c.sdkURL).
		Str("sdk_digest", c.sdkDigest)
	if c.trigger != nil {
		in = in.Computable("trigger", c.trigger)
	}
	if c.localfs != nil {
		in = in.Computable("localfs", c.localfs)
	} else {
		in = in.Indigestible("localfs", "not available")
	}
	return in
}

func (c *bazelRemoteCompilation) Compute(ctx context.Context, deps compute.Resolved) (_ fs.FS, retErr error) {
	targetDir, err := dirs.CreateUserTempDir("go", "build")
	if err != nil {
		return nil, err
	}
	stagingDir, err := dirs.CreateUserTempDir("go", "remote-input")
	if err != nil {
		return nil, err
	}
	defer func() {
		retErr = errors.Join(retErr, os.RemoveAll(stagingDir))
	}()

	if err := writeWorkspaceTar(filepath.Join(stagingDir, ".workspace.tar"), c.workspaceAbs); err != nil {
		return nil, fnerrors.Newf("failed to archive Go workspace: %w", err)
	}

	remoteExecutor, err := bazelremote.Executor(ctx)
	if err != nil {
		return nil, fnerrors.Newf("failed to connect to Bazel remote execution: %w", err)
	}
	sdkDigest, err := remoteExecutor.FetchBlob(ctx, c.sdkURL, c.sdkDigest)
	if err != nil {
		return nil, fnerrors.Newf("failed to fetch Go SDK into Bazel CAS: %w", err)
	}

	outputName := strings.TrimLeft(c.binary.BinaryName, `/\`)
	args := constructArgs(goBuildArgs(c.sdkVersion, c.binary.StripSymbols, c.binary.StripDwarf))
	args = append(args, filepath.ToSlash(filepath.Join(c.binary.GoModule, c.binary.SourcePath)))
	script := `set -euo pipefail; mkdir .workspace .go-sdk; tar -xf .workspace.tar -C .workspace; tar -xzf .go-sdk.tar.gz -C .go-sdk; export GOROOT="$PWD/.go-sdk/go"; exec "$GOROOT/bin/go" -C "$PWD/.workspace/` + filepath.ToSlash(c.binary.GoWorkspacePath) + `" build -o="$PWD/` + outputName + `" "$@"`
	cmd := &command.Command{
		Args:     append([]string{"/bin/bash", "-c", script, "go"}, args...),
		ExecRoot: stagingDir,
		InputSpec: &command.InputSpec{
			Inputs:        []string{".workspace.tar"},
			VirtualInputs: []*command.VirtualInput{{Path: ".go-sdk.tar.gz", Digest: sdkDigest.String()}},
			EnvironmentVariables: map[string]string{
				"CGO_ENABLED": "0",
				"GOOS":        c.platform.OS,
				"GOARCH":      c.platform.Architecture,
			},
		},
		OutputFiles: []string{outputName},
		Timeout:     10 * time.Minute,
		Platform: map[string]string{
			"OSFamily":                    "Linux",
			"Arch":                        "amd64",
			"namespace_requires_network":  "true",
			"namespace_pool":              "nsdev-go",
			"namespace_pool_machine_type": "linux/amd64:4x8",
			"namespace_pool_slots":        "8",
			"namespace_pool_duration":     "10m",
		},
	}
	cmd.FillDefaultFieldValues()

	oe := outerr.NewRecordingOutErr()
	// Run's second result is execution metadata; execution errors are reported by result.Err.
	result, _ := remoteExecutor.Run(ctx, cmd, command.DefaultExecutionOptions(), oe)
	if result.Err != nil {
		return nil, fnerrors.Newf("Bazel remote execution failed: %w", result.Err)
	}
	if !result.IsOk() {
		return nil, fnerrors.Newf("remote Go build exited with status %d: %s", result.ExitCode, strings.TrimSpace(string(oe.Stderr())))
	}
	if err := os.Rename(filepath.Join(stagingDir, outputName), filepath.Join(targetDir, outputName)); err != nil {
		return nil, fnerrors.Newf("failed to collect remote Go build output: %w", err)
	}
	built := fnfs.Local(targetDir)
	layer, err := oci.LayerFromFS(ctx, built)
	if err != nil {
		return nil, err
	}
	remoteUploadLayer, err := oci.WithBazelRemoteUpload(layer, remoteExecutor.Client)
	if err != nil {
		return nil, err
	}
	return bazelRemoteBuildFS{
		FS:    built,
		layer: remoteUploadLayer,
	}, nil
}

func writeWorkspaceTar(destination, root string) (retErr error) {
	output, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, output.Close())
	}()

	w := tar.NewWriter(output)
	defer func() {
		retErr = errors.Join(retErr, w.Close())
	}()

	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return fs.SkipDir
		}

		rel, err := filepath.Rel(root, path)
		if err != nil || rel == "." {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		link := ""
		if info.Mode()&fs.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		header.ModTime = time.Unix(1, 0)
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}
		header.Uid = 0
		header.Gid = 0
		header.Uname = ""
		header.Gname = ""
		if err := w.WriteHeader(header); err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, input)
		return errors.Join(copyErr, input.Close())
	})
}

type bazelRemoteBuildFS struct {
	fs.FS
	layer oci.Layer
}

func (fs bazelRemoteBuildFS) AsLayer() (v1.Layer, error) {
	return fs.layer, nil
}
