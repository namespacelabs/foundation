// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func main() {
	fncobra.DoMain("nsc", func(root *cobra.Command) {
		root.AddCommand(auth.NewLoginCmd())
		root.AddCommand(cluster.NewClusterCmd(false))
	})
}
