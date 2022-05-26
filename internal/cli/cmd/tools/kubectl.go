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
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
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

			kubectlBin, err := kubectl.EnsureSDK(ctx)
			if err != nil {
				return err
			}

			k8sconfig := k8s.KubeConfig()
			kubectl := exec.CommandContext(ctx, string(kubectlBin),
				append([]string{
					"--kubeconfig=" + k8sconfig.Config,
					"--context=" + k8sconfig.Context,
					"-n", k8sconfig.Namespace,
				}, args...)...)
			return localexec.RunInteractive(ctx, kubectl)
		}),
	}

	cmd.Flags().StringVar(&envRef, "env", envRef, "The environment to access (as defined in the workspace).")

	return cmd
}
