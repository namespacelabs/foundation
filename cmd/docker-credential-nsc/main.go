// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"github.com/spf13/cobra"
	ia "namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster/credhelper"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
)

func main() {
	fncobra.DoMain(fncobra.MainOpts{
		Name: "docker-credential-nsc",
		RegisterCommands: func(root *cobra.Command) {
			endpoint.SetupFlags("", root.PersistentFlags(), false)
			ia.SetupFlags(root.PersistentFlags())

			root.AddCommand(credhelper.NewDockerCredHelperStoreCmd(false))
			root.AddCommand(credhelper.NewDockerCredHelperGetCmd(false))
			root.AddCommand(credhelper.NewDockerCredHelperListCmd(false))
			root.AddCommand(credhelper.NewDockerCredHelperEraseCmd(false))
		},
	})
}
