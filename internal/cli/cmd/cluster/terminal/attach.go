package terminal

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
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
