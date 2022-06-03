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
)

func newKubeCtlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl",
		Short: "Run kubectl, configured for the specified environment.",
	}

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		k8s, err := kubernetes.NewFromEnv(ctx, env)
		if err != nil {
			return err
		}

		kubectlBin, err := kubectl.EnsureSDK(ctx)
		if err != nil {
			return err
		}

		k8sconfig := k8s.Bind(env.Workspace(), env.Proto()).KubeConfig()
		kubectl := exec.CommandContext(ctx, string(kubectlBin),
			append([]string{
				"--kubeconfig=" + k8sconfig.Config,
				"--context=" + k8sconfig.Context,
				"-n", k8sconfig.Namespace,
			}, args...)...)
		return localexec.RunInteractive(ctx, kubectl)
	})
}
