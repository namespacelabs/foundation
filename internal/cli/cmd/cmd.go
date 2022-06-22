// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/lsp"
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
	root.AddCommand(NewPrepareCmd())
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
	root.AddCommand(NewSdkCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewBundlesCmd())
	root.AddCommand(NewAttachCmd())
	root.AddCommand(NewDeploymentCmd())
	root.AddCommand(NewUseCmd())
	root.AddCommand(NewDeployPlanCmd())
	root.AddCommand(NewEksCmd())
	root.AddCommand(NewImagesCmd())
	root.AddCommand(lsp.NewLSPCmd())
	root.AddCommand(secrets.NewSecretsCmd())
	root.AddCommand(source.NewSourceCmd())
	root.AddCommand(tools.NewToolsCmd())
}
