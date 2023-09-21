// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package terminal

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/go-ids"
)

func newCreateCmd() *cobra.Command {
	run := &cobra.Command{
		Use:   "create",
		Short: "Creates a new instance that can be used as a terminal.",
		Args:  cobra.NoArgs,
	}

	image := run.Flags().String("image", "", "Which image to run.")
	machineType := run.Flags().String("machine_type", "", "Specify the machine type.")
	duration := run.Flags().Duration("duration", 0, "For how long to run the ephemeral environment.")

	run.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *image == "" {
			return fnerrors.New("--image is required")
		}

		opts := cluster.CreateContainerOpts{
			Name:            ids.NewRandomBase32ID(6),
			Image:           *image,
			Args:            []string{"sleep", "infinity"},
			EnableDocker:    true,
			ForwardNscState: true,
			Features:        []string{"EXP_USE_CONTAINER_AS_TERMINAL_SOURCE"},
		}

		resp, err := cluster.CreateContainerInstance(ctx, *machineType, *duration, "", false, opts)
		if err != nil {
			return err
		}

		return cluster.PrintCreateContainersResult(ctx, "plain", resp)
	})

	return run
}
