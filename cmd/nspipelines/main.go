// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/github"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/runs"
	workspaceCmd "namespacelabs.dev/foundation/cmd/nspipelines/cmd/workspace"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontendopaque"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
)

const maxLogLevel = 0

func main() {
	root := &cobra.Command{
		Use: "nspipelines",

		TraverseChildren: true,
	}

	root.AddCommand(github.NewGithubCmd())
	root.AddCommand(workspaceCmd.NewWorkspaceCmd())
	root.AddCommand(runs.NewRunsCmd())
	root.AddCommand(cmd.NewRobotLogin("robot-login"))

	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(os.Stderr, maxLogLevel))

	ecr.Register()
	workspace.ModuleLoader = cuefrontend.ModuleLoader

	workspace.MakeFrontend = func(pl workspace.EarlyPackageLoader, env *schema.Environment) workspace.Frontend {
		return cuefrontend.NewFrontend(pl, cuefrontendopaque.NewFrontend(env, pl), env)
	}

	tasks.SetupFlags(root.PersistentFlags())

	if err := root.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
