// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/module"
)

func newKubeCtlCmd() *cobra.Command {
	var envRef = "dev"

	cmd := &cobra.Command{
		Use:   "kubectl",
		Short: "Run kubectl, configured for the specified environment.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, envRef)
			if err != nil {
				return err
			}

			k8s, err := kubernetes.New(ctx, root.Workspace, root.DevHost, env.Proto())
			if err != nil {
				return err
			}

			return k8s.Kubectl(ctx, rtypes.IO{
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Stdin:  os.Stdin,
			}, args...)
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to access (as defined in the workspace).")

	return cmd
}
