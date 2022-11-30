// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/cfg"
)

func newKubeCtlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl, configured for the specified environment.",
	}

	keepConfig := cmd.Flags().Bool("keep_config", false, "If set to true, does not delete the generated configuration.")

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env cfg.Context, args []string) error {
		cfg, err := writeKubeconfig(ctx, env, *keepConfig)
		if err != nil {
			return err
		}

		defer func() {
			_ = cfg.Cleanup()
		}()

		cmdLine := append(cfg.BaseArgs(), args...)

		if *keepConfig {
			fmt.Fprintf(console.Stderr(ctx), "Running kubectl %s\n", strings.Join(cmdLine, " "))
		}

		kubectlBin, err := kubectl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download Kubernetes SDK: %w", err)
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

func writeKubeconfig(ctx context.Context, env cfg.Context, keepConfig bool) (*Kubeconfig, error) {
	cluster, err := kubernetes.ConnectToNamespace(ctx, env)
	if err != nil {
		return nil, err
	}

	kluster := cluster.Cluster().(*kubernetes.Cluster)

	k8sconfig := cluster.KubeConfig()
	rawConfig, err := kluster.PreparedClient().ClientConfig.RawConfig()
	if err != nil {
		return nil, fnerrors.New("failed to generate kubeconfig: %w", err)
	}

	c, err := WriteRawKubeconfig(ctx, rawConfig, keepConfig)
	if err != nil {
		return nil, err
	}

	c.Namespace = k8sconfig.Namespace
	c.Context = k8sconfig.Context
	return c, nil
}

func WriteRawKubeconfig(ctx context.Context, rawConfig clientcmdapi.Config, keepConfig bool) (*Kubeconfig, error) {
	configBytes, err := clientcmd.Write(rawConfig)
	if err != nil {
		return nil, fnerrors.New("failed to serialize kubeconfig: %w", err)
	}

	tmpFile, err := dirs.CreateUserTemp("kubeconfig", "*.yaml")
	if err != nil {
		return nil, fnerrors.New("failed to create temp file: %w", err)
	}

	if _, err := tmpFile.Write(configBytes); err != nil {
		return nil, fnerrors.New("failed to write kubeconfig: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return nil, fnerrors.New("failed to close kubeconfig: %w", err)
	}

	return &Kubeconfig{
		Kubeconfig: tmpFile.Name(),
		keepConfig: keepConfig,
	}, nil
}

func (kc *Kubeconfig) BaseArgs() []string {
	baseArgs := []string{
		"--kubeconfig=" + kc.Kubeconfig,
	}

	if kc.Namespace != "" {
		baseArgs = append(baseArgs, "-n", kc.Namespace)
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
