// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func newKubeCtlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl, configured for the specified environment.",
	}

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		k8s, err := kubernetes.NewFromEnv(ctx, env)
		if err != nil {
			return err
		}

		kubectlBin, err := kubectl.EnsureSDK(ctx)
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to download Kubernetes SDK")
		}

		runtime := k8s.Bind(env.Workspace(), env.Proto())
		clientConfig := client.NewClientConfig(ctx, runtime.HostConfig())
		rawConfig, err := clientConfig.RawConfig()
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to generate kubeconfig")
		}
		configBytes, err := clientcmd.Write(rawConfig)
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to serialize kubeconfig")
		}
		tmpFile, err := dirs.CreateUserTemp("kubeconfig", "*.yaml")
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to create temp file")
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.Write(configBytes); err != nil {
			return fnerrors.Wrapf(nil, err, "failed to write kubeconfig")
		}
		if err := tmpFile.Close(); err != nil {
			return fnerrors.Wrapf(nil, err, "failed to close kubeconfig")
		}

		ns, _, err := clientConfig.Namespace()
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to determine namespace")
		}
		cmdLine := append([]string{
			"--kubeconfig=" + tmpFile.Name(),
			"-n", ns,
		}, args...)
		kubectl := exec.CommandContext(ctx, string(kubectlBin), cmdLine...)
		return localexec.RunInteractive(ctx, kubectl)
	})
}
