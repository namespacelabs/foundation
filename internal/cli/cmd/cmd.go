// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/create"
	"namespacelabs.dev/foundation/internal/cli/cmd/eks"
	"namespacelabs.dev/foundation/internal/cli/cmd/lsp"
	"namespacelabs.dev/foundation/internal/cli/cmd/prepare"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/secrets"
	"namespacelabs.dev/foundation/internal/cli/cmd/source"
	"namespacelabs.dev/foundation/internal/cli/cmd/tools"
)

func RegisterCommands(root *cobra.Command) {
	root.AddCommand(NewLintCmd())
	root.AddCommand(NewBuildCmd())
	root.AddCommand(NewLsCmd())
	root.AddCommand(NewGenerateCmd())
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
	root.AddCommand(NewBundlesCmd())
	root.AddCommand(NewAttachCmd())
	root.AddCommand(NewDeploymentCmd())
	root.AddCommand(NewUseCmd())
	root.AddCommand(NewDeployPlanCmd())
	root.AddCommand(NewImagesCmd())
	root.AddCommand(eks.NewEksCmd())
	root.AddCommand(lsp.NewLSPCmd())
	root.AddCommand(prepare.NewPrepareCmd())
	root.AddCommand(secrets.NewSecretsCmd())
	root.AddCommand(source.NewSourceCmd())
	root.AddCommand(tools.NewToolsCmd())
	root.AddCommand(create.NewCreateCmd(RunCommand))
	root.AddCommand(cluster.NewClusterCmd())
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
