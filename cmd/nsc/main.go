// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain("nsc", false, func(root *cobra.Command) {
		api.SetupFlags("", root.PersistentFlags(), false)

		root.AddCommand(auth.NewAuthCmd())
		root.AddCommand(auth.NewLoginCmd()) // register `nsc login` as an alias for `nsc auth login`

		root.AddCommand(version.NewVersionCmd())

		root.AddCommand(cluster.NewBareClusterCmd(false))
		root.AddCommand(cluster.NewKubectlCmd())          // nsc kubectl
		root.AddCommand(cluster.NewBuildkitCmd())         // nsc buildkit builctl
		root.AddCommand(cluster.NewBuildCmd())            // nsc build
		root.AddCommand(cluster.NewDockerLoginCmd(false)) // nsc docker-login
		root.AddCommand(cluster.NewMetadataCmd())         // nsc metadata
		root.AddCommand(cluster.NewCreateCmd())           // nsc create
		root.AddCommand(cluster.NewListCmd())             // nsc list
		root.AddCommand(cluster.NewLogsCmd())             // nsc logs
		root.AddCommand(cluster.NewExposeCmd())           // nsc expose
		root.AddCommand(cluster.NewRunCmd())              // nsc run
		root.AddCommand(cluster.NewRunComposeCmd())       // nsc run-compose
		root.AddCommand(cluster.NewSshCmd())              // nsc ssh

		root.AddCommand(sdk.NewSdkCmd(true))

		fncobra.PushPreParse(root, func(ctx context.Context, args []string) error {
			api.Register()
			return nil
		})
	})
}
