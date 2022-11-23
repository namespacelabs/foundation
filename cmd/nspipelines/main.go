// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
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
	root.AddCommand(cmd.NewImagesCmd())
	root.AddCommand(cmd.NewRobotLogin("robot-login"))

	ctx := tasks.WithSink(context.Background(), simplelog.NewSink(os.Stderr, maxLogLevel))

	ecr.Register()
	parsing.ModuleLoader = cuefrontend.ModuleLoader

	parsing.MakeFrontend = func(pl parsing.EarlyPackageLoader, env *schema.Environment) parsing.Frontend {
		return cuefrontend.NewFrontend(pl, cuefrontendopaque.NewFrontend(env, pl), env)
	}

	tasks.SetupFlags(root.PersistentFlags())

	if err := root.ExecuteContext(ctx); err != nil {
		log.Fatal(err)
	}
}
