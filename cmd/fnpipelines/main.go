// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/cmd/fnpipelines/cmd/github"
	"namespacelabs.dev/foundation/cmd/fnpipelines/cmd/workspace"
)

func main() {
	root := &cobra.Command{
		Use: "fnpipelines",

		TraverseChildren: true,
	}

	root.AddCommand(github.NewGithubCmd())
	root.AddCommand(workspace.NewWorkspaceCmd())

	if err := root.ExecuteContext(context.Background()); err != nil {
		log.Fatal(err)
	}
}
