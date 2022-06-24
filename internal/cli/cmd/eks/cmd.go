// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import "github.com/spf13/cobra"

func NewEksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "eks",
		Short:  "EKS-related activities (internal only).",
		Hidden: true,
	}

	cmd.AddCommand(newComputeIrsaCmd())
	cmd.AddCommand(newGenerateTokenCmd())
	cmd.AddCommand(newGenerateConfigCmd())
	cmd.AddCommand(NewSetupAutopushCmd())

	return cmd
}
