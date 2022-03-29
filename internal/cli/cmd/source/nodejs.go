// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"os"
	"path/filepath"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func newNodejsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run nodejs.",

		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodejs(cmd.Context(), "node", args...)
		},
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

	res, err := compute.Get(ctx, oci.ResolveImage("node:16.13", rt.HostPlatform()))
	if err != nil {
		return err
	}

	return rt.Run(ctx, rtypes.RunToolOpts{
		IO:          rtypes.StdIO(ctx),
		AllocateTTY: true,
		Mounts:      []*rtypes.LocalMapping{{HostPath: root.Abs(), ContainerPath: "/workspace"}},
		RunBinaryOpts: rtypes.RunBinaryOpts{
			Image:      res.Value.(v1.Image),
			WorkingDir: filepath.Join("/workspace", rel),
			Command:    []string{command},
			Args:       args,
		}})
}