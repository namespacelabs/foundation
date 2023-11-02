// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewKubeCtlCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kubectl -- ...",
		Short:   "Run kubectl, configured for the specified environment.",
		Example: "ns tools kubectl --env=dev get pod -- -A",
		Hidden:  hidden,
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

func NewWriteKubeConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write-kubeconfig",
		Short: "Emits a kubeconfig for the specified environment.",
	}

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env cfg.Context, args []string) error {
		cfg, err := writeKubeconfig(ctx, env, true)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "%s\n", cfg.Kubeconfig)
		return nil
	})
}

func writeKubeconfig(ctx context.Context, env cfg.Context, keepConfig bool) (*kubectl.Kubeconfig, error) {
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

	c, err := kubectl.WriteRawKubeconfig(ctx, rawConfig, keepConfig)
	if err != nil {
		return nil, err
	}

	c.Namespace = k8sconfig.Namespace
	c.Context = k8sconfig.Context
	return c, nil
}
