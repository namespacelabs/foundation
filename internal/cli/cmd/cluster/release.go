// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newSuspendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "suspend [cluster-id]",
		Short:  "Suspends an existing cluster.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return api.Endpoint.SuspendKubernetesCluster.Do(ctx, api.SuspendKubernetesClusterRequest{
			ClusterId: args[0],
		}, nil)
	})

	return cmd
}

func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "release [cluster-id]",
		Short:  "Releases an existing cluster.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return api.Endpoint.ReleaseKubernetesCluster.Do(ctx, api.ReleaseKubernetesClusterRequest{
			ClusterId: args[0],
		}, nil)
	})

	return cmd
}

func newWakeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "wake [cluster-id]",
		Short:  "Wakes up an existing cluster.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return api.Endpoint.WakeKubernetesCluster.Do(ctx, api.WakeKubernetesClusterRequest{
			ClusterId: args[0],
		}, nil)
	})

	return cmd
}
