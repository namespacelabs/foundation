// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/source/create"
)

func NewSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Short:   "Source-related commands.",
		Aliases: []string{"src"},
	}

	cmd.AddCommand(newBufCmd())
	cmd.AddCommand(newBufGenerateCmd())
	cmd.AddCommand(create.NewCreateCmd())
	cmd.AddCommand(newNodejsCmd())
	cmd.AddCommand(newYarnCmd())
	cmd.AddCommand(newNewIdCmd())

	return cmd
}
