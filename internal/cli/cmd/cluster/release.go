// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
)

func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "release [instance-id]",
		Short:  "Releases an existing instance.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return api.Methods.ReleaseKubernetesCluster.Do(ctx, api.ReleaseKubernetesClusterRequest{
			ClusterId: args[0],
		}, endpoint.ResolveRegionalEndpoint, nil)
	})

	return cmd
}
