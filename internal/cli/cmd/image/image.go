package image

import "github.com/spf13/cobra"

func NewImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "image",
		Short:  "Internal commands, for debugging.",
		Hidden: true,
	}

	cmd.AddCommand(newMakeDiskCmd())

	return cmd
}
