// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/create"
	"namespacelabs.dev/foundation/internal/cli/cmd/lsp"
	"namespacelabs.dev/foundation/internal/cli/cmd/prepare"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/secrets"
	"namespacelabs.dev/foundation/internal/cli/cmd/tools"
)

func RegisterCommands(root *cobra.Command) {
	root.AddCommand(NewBuildCmd())
	root.AddCommand(NewLsCmd())
	root.AddCommand(NewDeployCmd())
	root.AddCommand(NewFmtCmd())
	root.AddCommand(NewUnprepareCmd())
	root.AddCommand(NewDevCmd())
	root.AddCommand(NewBuildBinaryCmd())
	root.AddCommand(NewCacheCmd())
	root.AddCommand(NewTidyCmd())
	root.AddCommand(NewLogsCmd())
	root.AddCommand(NewLoginCmd())
	root.AddCommand(NewKeysCmd())
	root.AddCommand(NewTestCmd())
	root.AddCommand(NewDebugShellCmd())
	root.AddCommand(NewModCmd())
	root.AddCommand(sdk.NewSdkCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewAttachCmd())
	root.AddCommand(NewDeploymentCmd())
	root.AddCommand(NewDeployPlanCmd())
	root.AddCommand(lsp.NewLSPCmd())
	root.AddCommand(prepare.NewPrepareCmd())
	root.AddCommand(secrets.NewSecretsCmd())
	root.AddCommand(tools.NewToolsCmd())
	root.AddCommand(create.NewCreateCmd(RunCommand))
	root.AddCommand(NewGenerateCmd())
}

// Programmatically trigger an `ns` command.
func RunCommand(ctx context.Context, args []string) error {
	root := &cobra.Command{
		TraverseChildren: true,
	}
	RegisterCommands(root)

	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}
