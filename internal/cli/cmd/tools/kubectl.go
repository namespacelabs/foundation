// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func newKubeCtlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl, configured for the specified environment.",
	}

	keepConfig := cmd.Flags().Bool("keep_config", false, "If set to true, does not delete the generated configuration.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env planning.Context, args []string) error {
		cfg, err := writeKubeconfig(ctx, env, *keepConfig)
		if err != nil {
			return err
		}

		defer cfg.Cleanup()

		cmdLine := append(cfg.BaseArgs(), args...)

		if *keepConfig {
			fmt.Fprintf(console.Stderr(ctx), "Running kubectl %s\n", strings.Join(cmdLine, " "))
		}

		kubectlBin, err := kubectl.EnsureSDK(ctx)
		if err != nil {
			return fnerrors.Wrapf(nil, err, "failed to download Kubernetes SDK")
		}

		kubectl := exec.CommandContext(ctx, string(kubectlBin), cmdLine...)
		return localexec.RunInteractive(ctx, kubectl)
	})
}

type Kubeconfig struct {
	Kubeconfig string
	Context    string
	Namespace  string
	keepConfig bool
}

func writeKubeconfig(ctx context.Context, env planning.Context, keepConfig bool) (*Kubeconfig, error) {
	cluster, err := kubernetes.ConnectToNamespace(ctx, env)
	if err != nil {
		return nil, err
	}

	kluster := cluster.Cluster().(*kubernetes.Cluster)

	k8sconfig := cluster.KubeConfig()
	rawConfig, err := kluster.PreparedClient().ClientConfig.RawConfig()
	if err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to generate kubeconfig")
	}

	configBytes, err := clientcmd.Write(rawConfig)
	if err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to serialize kubeconfig")
	}

	tmpFile, err := dirs.CreateUserTemp("kubeconfig", "*.yaml")
	if err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to create temp file")
	}

	if _, err := tmpFile.Write(configBytes); err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to write kubeconfig")
	}

	if err := tmpFile.Close(); err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to close kubeconfig")
	}

	return &Kubeconfig{
		Kubeconfig: tmpFile.Name(),
		Namespace:  k8sconfig.Namespace,
		Context:    k8sconfig.Context,
		keepConfig: keepConfig,
	}, nil
}

func (kc *Kubeconfig) BaseArgs() []string {
	baseArgs := []string{
		"--kubeconfig=" + kc.Kubeconfig,
		"-n", kc.Namespace,
	}

	if kc.Context != "" {
		baseArgs = append(baseArgs, "--context", kc.Context)
	}

	return baseArgs
}

func (kc *Kubeconfig) Cleanup() error {
	if kc.keepConfig {
		return nil
	}

	return os.Remove(kc.Kubeconfig)
}
