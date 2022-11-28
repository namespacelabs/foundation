// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"context"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/dependencies/pins"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const (
	workspaceContainerDir = "/workspace"
)

type RunNodejsOpts struct {
	Scope         schema.PackageName
	Args          []string
	EnvVars       []*schema.BinaryConfig_EnvEntry
	Mounts        []*rtypes.LocalMapping
	IsInteractive bool
}

func RunNodejs(ctx context.Context, env cfg.Context, relPath string, command string, opts *RunNodejsOpts) error {
	p, err := tools.HostPlatform(ctx, env.Configuration())
	if err != nil {
		return err
	}

	nodeImageName, err := pins.CheckDefault("node")
	if err != nil {
		return err
	}

	nodeImageState := llbutil.Image(nodeImageName, p)

	nodejsImage, err := buildkit.BuildImage(ctx, env, build.NewBuildTarget(&p).WithSourceLabel("nodejs: %s", nodeImageName), nodeImageState)
	if err != nil {
		return err
	}

	image, err := compute.GetValue(ctx, nodejsImage)
	if err != nil {
		return err
	}

	var io rtypes.IO
	if opts.IsInteractive {
		done := console.EnterInputMode(ctx)
		defer done()
		io = rtypes.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}
	} else if opts.Scope != "" {
		stdout := console.Output(ctx, console.MakeConsoleName(opts.Scope.String(), "nodejs", ""))
		io = rtypes.IO{Stdout: stdout, Stderr: stdout}
	} else {
		stdout := console.Output(ctx, "nodejs")
		io = rtypes.IO{Stdout: stdout, Stderr: stdout}
	}

	abs := env.Workspace().LoadedFrom().AbsPath

	return tools.Run(ctx, env.Configuration(), rtypes.RunToolOpts{
		IO:          io,
		AllocateTTY: opts.IsInteractive,
		Mounts: append(opts.Mounts, &rtypes.LocalMapping{
			HostPath: abs,
			// The user's filesystem structure is replicated within the container.
			ContainerPath: filepath.Join(workspaceContainerDir, abs),
		}),
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      image,
			WorkingDir: filepath.Join(workspaceContainerDir, abs, relPath),
			Command:    []string{command},
			Args:       opts.Args,
			Env:        opts.EnvVars,
		}})
}
