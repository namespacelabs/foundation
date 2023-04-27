// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"github.com/spf13/cobra"
	ia "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func main() {
	fncobra.DoMain("docker-credential-nsc", false, func(root *cobra.Command) {
		api.SetupFlags("", root.PersistentFlags(), false)
		ia.SetupFlags(root.PersistentFlags())

		root.AddCommand(cluster.NewDockerCredHelperStoreCmd(false))
		root.AddCommand(cluster.NewDockerCredHelperGetCmd(false))
		root.AddCommand(cluster.NewDockerCredHelperListCmd(false))
		root.AddCommand(cluster.NewDockerCredHelperEraseCmd(false))

		fncobra.PushPreParse(root, func(ctx context.Context, args []string) error {
			api.Register()
			return nil
		})
	})
}
