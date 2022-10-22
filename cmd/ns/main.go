// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func main() {
	fncobra.DoMain("ns", func(root *cobra.Command) {
		cmd.RegisterCommands(root)
	})
}
