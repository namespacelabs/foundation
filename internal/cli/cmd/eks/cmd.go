// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	cmd.AddCommand(newSetupAutopushCmd())
	cmd.AddCommand(newPrepareCmd())

	return cmd
}
