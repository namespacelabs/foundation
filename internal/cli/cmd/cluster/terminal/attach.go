// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package terminal

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newAttachCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "attach [cluster-id]",
		Short: "Attaches to a terminal source.",
		Args:  cobra.MaximumNArgs(1),
	}

	sshAgent := cmd.Flags().BoolP("ssh_agent", "A", false, "If specified, forwards the local SSH agent.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		c, _, err := cluster.SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, cluster.ErrEmptyClusterList) {
				cluster.PrintCreateClusterMsg(ctx)
				return nil
			}

			return err
		}

		if c == nil {
			return nil
		}

		return cluster.InlineSsh(ctx, c, *sshAgent, []string{"nsc", "internal", "attach", "/bin/bash"})
	})

	return cmd
}

func newRunScriptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-script [cluster-id]",
		Short: "Runs specified script on a terminal source.",
		Args:  cobra.MaximumNArgs(1),
	}

	scriptFile := cmd.Flags().StringP("file", "f", "", "The script file to run.")
	sshAgent := cmd.Flags().BoolP("ssh_agent", "A", false, "If specified, forwards the local SSH agent.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *scriptFile == "" {
			return fnerrors.New("--file/-f is required")
		}

		contents, err := os.ReadFile(*scriptFile)
		if err != nil {
			return err
		}

		c, _, err := cluster.SelectRunningCluster(ctx, args)
		if err != nil {
			if errors.Is(err, cluster.ErrEmptyClusterList) {
				cluster.PrintCreateClusterMsg(ctx)
				return nil
			}

			return err
		}

		if c == nil {
			return nil
		}

		return cluster.InlineSsh(ctx, c, *sshAgent, []string{"nsc", "internal", "attach", "--", "/bin/bash", "-c", string(contents)})
	})

	return cmd
}
