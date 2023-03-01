// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func main() {
	// Consider adding auto updates if we frequently change nsc.
	fncobra.DoMain("nsc", false, func(root *cobra.Command) {
		api.SetupFlags(root.PersistentFlags(), false)

		root.AddCommand(auth.NewLoginCmd())
		root.AddCommand(cluster.NewClusterCmd(false))
		root.AddCommand(cluster.NewKubectlCmd()) // `nsc kubectl` acts as an alias for `nsc cluster kubectl`
		root.AddCommand(version.NewVersionCmd())

		fncobra.PushPreParse(root, func(ctx context.Context, args []string) error {
			api.Register()
			return nil
		})
	})
}
