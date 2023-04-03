// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/metadata"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain("nsc", false, func(root *cobra.Command) {
		api.SetupFlags(root.PersistentFlags(), false)

		root.AddCommand(auth.NewAuthCmd())
		root.AddCommand(auth.NewLoginCmd()) // register `nsc login` as an alias for `nsc auth login`

		root.AddCommand(version.NewVersionCmd())

		root.AddCommand(cluster.NewClusterCmd(false))
		root.AddCommand(cluster.NewKubectlCmd())          // `nsc kubectl` acts as an alias for `nsc cluster kubectl`
		root.AddCommand(cluster.NewBuildctlCmd())         // `nsc buildctl` acts as an alias for `nsc cluster buildctl`
		root.AddCommand(cluster.NewBuildCmd())            // `nsc build` acts as an alias for `nsc cluster build`
		root.AddCommand(cluster.NewDockerLoginCmd(false)) // `nsc docker-login` acts as an alias for `nsc cluster docker-login`
		root.AddCommand(metadata.NewMetadataCmd())

		root.AddCommand(sdk.NewSdkCmd(true))

		fncobra.PushPreParse(root, func(ctx context.Context, args []string) error {
			api.Register()
			return nil
		})
	})
}
