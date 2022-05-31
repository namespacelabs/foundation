// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func newNodejsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run nodejs.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			return runNodejs(ctx, "node", args...)
		}),
	}

	return cmd
}

func runNodejs(ctx context.Context, command string, args ...string) error {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(root.Abs(), cwd)
	if err != nil {
		return err
	}

	rt := tools.Impl()

	image, err := compute.GetValue(ctx, oci.ResolveImage("node:16.13", rt.HostPlatform()))
	if err != nil {
		return err
	}

	done := console.EnterInputMode(ctx)
	defer done()

	return tools.Run(ctx, rtypes.RunToolOpts{
		IO:          rtypes.IO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr},
		AllocateTTY: true,
		Mounts:      []*rtypes.LocalMapping{{HostPath: root.Abs(), ContainerPath: "/workspace"}},
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      image,
			WorkingDir: filepath.Join("/workspace", rel),
			Command:    []string{command},
			Args:       args,
		}})
}
