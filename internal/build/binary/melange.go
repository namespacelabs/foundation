// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/melange"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type melangeBuild struct {
	rel string

	files     []string
	platforms []string
}

func (m melangeBuild) BuildImage(ctx context.Context, env pkggraph.SealedContext, conf build.Configuration) (compute.Computable[oci.Image], error) {
	action := tasks.Action("melange.build").Scope(conf.SourcePackage())

	mbin, err := melange.SDK(ctx, host.HostPlatform())
	if err != nil {
		return nil, err
	}

	localfs := memfs.DeferSnapshot(conf.Workspace().ReadOnlyFS(m.rel), memfs.SnapshotOpts{
		IncludeFiles: m.files,
	})

	return compute.Map(action, compute.Inputs().Computable("mbin", mbin).JSON("platform", m.platforms).Computable("localfs", localfs), compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (oci.Image, error) {
			mb := compute.MustGetDepValue(deps, mbin, "mbin")
			contents := compute.MustGetDepValue(deps, localfs, "localfs")

			dir, err := os.MkdirTemp("", "melange")
			if err != nil {
				return nil, err
			}

			fmt.Fprintf(console.Debug(ctx), "melange: created %s\n", dir)

			defer os.RemoveAll(dir)

			for _, path := range m.files {
				c, err := fs.ReadFile(contents, path)
				if err != nil {
					return nil, err
				}

				if err := os.WriteFile(filepath.Join(dir, path), c, 0660); err != nil {
					return nil, err
				}
			}

			out := console.Output(ctx, "melange")

			cmd := exec.CommandContext(ctx, string(mb),
				append([]string{
					"build",
					"-k", "https://packages.wolfi.dev/os/wolfi-signing.rsa.pub",
					"-r", "https://packages.wolfi.dev/os",
					"--arch", strings.Join(m.platforms, ","),
				}, m.files...)...,
			)
			cmd.Stdout = out
			cmd.Stderr = out
			cmd.Dir = dir

			if err := cmd.Run(); err != nil {
				return nil, err
			}

			layer, err := oci.LayerFromFS(ctx, os.DirFS(filepath.Join(dir, "packages")))
			if err != nil {
				return nil, err
			}

			return mutate.AppendLayers(empty.Image, layer)
		}), nil
}

func (m melangeBuild) PlatformIndependent() bool { return true }

func (m melangeBuild) Description() string { return fmt.Sprintf("melangeBuild()") }
