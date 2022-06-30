// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"context"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/pins"
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

func RunNodejs(ctx context.Context, env provision.Env, relPath string, command string, opts *RunNodejsOpts) error {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	p, err := tools.HostPlatform(ctx)
	if err != nil {
		return err
	}

	nodeImageName, err := pins.CheckDefault("node")
	if err != nil {
		return err
	}

	// TODO: generate a prebuilt
	nodeImageState, err := prepareNodejsBaseWithYarn(ctx, nodeImageName, p)
	if err != nil {
		return err
	}

	nodejsImage, err := buildkit.LLBToImage(ctx, env, build.NewBuildTarget(&p).WithSourceLabel("nodejs-with-yarn"), nodeImageState)
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
		stdout := console.Output(ctx, console.MakeConsoleName(opts.Scope.String(), "yarn", ""))
		io = rtypes.IO{Stdout: stdout, Stderr: stdout}
	} else {
		stdout := console.Output(ctx, "yarn")
		io = rtypes.IO{Stdout: stdout, Stderr: stdout}
	}

	return tools.Run(ctx, rtypes.RunToolOpts{
		IO:          io,
		AllocateTTY: opts.IsInteractive,
		Mounts:      append(opts.Mounts, &rtypes.LocalMapping{HostPath: root.Abs(), ContainerPath: workspaceContainerDir}),
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      image,
			WorkingDir: filepath.Join(workspaceContainerDir, relPath),
			Command:    []string{command},
			Args:       opts.Args,
			Env:        opts.EnvVars,
		}})
}
