// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/create"
	"namespacelabs.dev/foundation/internal/cli/cmd/eks"
	"namespacelabs.dev/foundation/internal/cli/cmd/lsp"
	"namespacelabs.dev/foundation/internal/cli/cmd/mod"
	"namespacelabs.dev/foundation/internal/cli/cmd/prepare"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/secrets"
	"namespacelabs.dev/foundation/internal/cli/cmd/tools"
	"namespacelabs.dev/foundation/internal/compute"
)

func RegisterCommands(root *cobra.Command) {
	root.AddCommand(NewBuildCmd())
	root.AddCommand(NewLsCmd())
	root.AddCommand(NewDeployCmd())
	root.AddCommand(NewDoctorCmd())
	root.AddCommand(NewFmtCmd())
	root.AddCommand(NewUnprepareCmd())
	root.AddCommand(NewDevCmd())
	root.AddCommand(NewDescribeCmd())
	root.AddCommand(NewBuildBinaryCmd())
	root.AddCommand(NewCacheCmd())
	root.AddCommand(mod.NewModCmd(RunCommand))
	root.AddCommand(mod.NewTidyCmd()) // register `ns tidy` as an alias for `ns mod tidy`
	root.AddCommand(NewLogsCmd())
	root.AddCommand(auth.NewAuthCmd())
	root.AddCommand(auth.NewLoginCmd())               // register `ns login` as an alias for `ns auth login`
	root.AddCommand(auth.NewExchangeGithubTokenCmd()) // register `ns exchange-github-token` as an alias for `ns auth exchange-github-token` to support old nscloud action versions. TODO remove
	root.AddCommand(NewKeysCmd())
	root.AddCommand(NewTestCmd())
	root.AddCommand(NewDebugShellCmd())
	root.AddCommand(sdk.NewSdkCmd())
	root.AddCommand(NewVersionCmd())
	root.AddCommand(NewAttachCmd())
	root.AddCommand(NewDeploymentCmd())
	root.AddCommand(NewDeployPlanCmd())
	root.AddCommand(eks.NewEksCmd())
	root.AddCommand(lsp.NewLSPCmd())
	root.AddCommand(prepare.NewPrepareCmd())
	root.AddCommand(prepare.NewPrepareIngressCmd())
	root.AddCommand(prepare.NewPrepareBuildClusterCmd())
	root.AddCommand(secrets.NewSecretsCmd())
	root.AddCommand(tools.NewToolsCmd())
	root.AddCommand(tools.NewKubeCtlCmd(true))
	root.AddCommand(create.NewCreateCmd(RunCommand))
	root.AddCommand(NewUpdateNSCmd())
	root.AddCommand(NewGenerateCmd())
	root.AddCommand(NewConfigCmd())
	root.AddCommand(cluster.NewClusterCmd())
}

// Programmatically trigger an `ns` command.
func RunCommand(ctx context.Context, args []string) error {
	root := &cobra.Command{
		TraverseChildren: true,
	}
	RegisterCommands(root)

	root.SetArgs(args)

	if compute.On(ctx) == nil {
		panic("no graph in context")
	}

	return root.ExecuteContext(ctx)
}
