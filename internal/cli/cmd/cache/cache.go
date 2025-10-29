package cache

import (
	"github.com/spf13/cobra"
)

func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache related commands.",
	}

	cmd.AddCommand(newModeCmd())

	return cmd
}
