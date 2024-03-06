package admin

import (
	"github.com/spf13/cobra"
)

func NewAdminCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "admin",
		Short:  "Partner administration commands.",
		Hidden: hidden,
	}

	cmd.AddCommand(newSignPartnerTokenCmd())

	return cmd
}
