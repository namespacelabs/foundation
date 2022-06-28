// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import "github.com/spf13/cobra"

func NewRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "runs",
	}

	cmd.AddCommand(newUploadCmd())
	cmd.AddCommand(newWriteCmd())

	return cmd
}
