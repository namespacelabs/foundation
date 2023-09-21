// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newExtendDurationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extend-duration [cluster-id]",
		Short: "Extends the duration of a cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	duration := cmd.Flags().Duration("duration", 0, "For how long to extend the ephemeral environment.")
	ensureMinimum := cmd.Flags().Duration("ensure_minimum", 0, "Ensure that the target instance has a minimum of the specified duration.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *duration == 0 && *ensureMinimum == 0 {
			return fnerrors.New("--duration or --ensure_minimum is required")
		}

		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		req := api.RefreshKubernetesClusterRequest{
			ClusterId:         cluster.ClusterId,
			ExtendBySecs:      int32(math.Floor(duration.Seconds())),
			EnsureMinimumSecs: int32(math.Floor(ensureMinimum.Seconds())),
		}

		resp, err := api.RefreshCluster(ctx, api.Methods, req)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "New deadline: %v\n", resp.NewDeadline)
		return nil
	})

	return cmd

}
