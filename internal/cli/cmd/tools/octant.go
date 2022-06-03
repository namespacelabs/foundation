// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/octant"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
)

func newOctantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "octant",
		Short: "[Experimental] Run Octant, configured for the specified environment.",
	}

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		k8s, err := kubernetes.New(ctx, env.DevHost(), env.Proto())
		if err != nil {
			return err
		}

		bin, err := octant.EnsureSDK(ctx)
		if err != nil {
			return err
		}

		cfg := k8s.Bind(env.Workspace()).KubeConfig()

		return localexec.RunInteractive(ctx, exec.CommandContext(ctx, string(bin), "--context="+cfg.Context, "--kubeconfig="+cfg.Config, "-n", cfg.Namespace))
	})
}
