// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/sdk/kubectl"
)

func NewKubectlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl -- ...",
		Short: "Run kubectl on the target cluster.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusterName := args[0]
		args = args[1:]

		response, err := api.GetCluster(ctx, api.Endpoint, clusterName)
		if err != nil {
			return err
		}

		cluster := response.Cluster

		cfg, err := kubectl.WriteRawKubeconfig(ctx, ctl.MakeConfig(cluster), false)
		if err != nil {
			return err
		}

		defer func() { _ = cfg.Cleanup() }()

		cmdLine := append(cfg.BaseArgs(), args...)

		kubectlBin, err := kubectl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download Kubernetes SDK: %w", err)
		}

		kubectl := exec.CommandContext(ctx, string(kubectlBin), cmdLine...)
		return localexec.RunInteractive(ctx, kubectl)
	})

	return cmd
}

func newKubeconfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeconfig",
		Short: "Write Kubeconfig for the target cluster.",
	}

	outputPath := cmd.Flags().String("output_to", "", "If specified, write the path of the Kubeconfig to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, err := selectCluster(ctx, args)
		if err != nil {
			if errors.Is(err, ErrEmptyClusteList) {
				printCreateClusterMsg(ctx)
				return nil
			}
			return err
		}

		if cluster == nil {
			return nil
		}

		cfg, err := kubectl.WriteRawKubeconfig(ctx, ctl.MakeConfig(cluster), false)
		if err != nil {
			return err
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(cfg.Kubeconfig), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}
		}

		fmt.Fprintf(console.Stdout(ctx), "Wrote Kubeconfig for cluster %s to %s.\n", cluster.ClusterId, cfg.Kubeconfig)
		return nil
	})

	return cmd
}
