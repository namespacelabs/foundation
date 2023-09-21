package private

import "github.com/spf13/cobra"

func NewInternalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "internal",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	cmd.AddCommand(newInternalTerminalAttach())

	return cmd
}
