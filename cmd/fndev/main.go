// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/cli/cmd/create"
	"namespacelabs.dev/foundation/internal/cli/cmd/debug"
	"namespacelabs.dev/foundation/internal/cli/cmd/source"
	"namespacelabs.dev/foundation/internal/cli/cmd/tools"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func main() {
	fncobra.DoMain("fndev", func(root *cobra.Command) {
		cmd.RegisterCommands(root)
		root.AddCommand(create.NewCreateCmd())
		root.AddCommand(debug.NewDebugCmd())
		root.AddCommand(source.NewSourceCmd())
		root.AddCommand(tools.NewToolsCmd())
	})
}
