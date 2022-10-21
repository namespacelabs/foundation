// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/github"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/runs"
	workspaceCmd "namespacelabs.dev/foundation/cmd/nspipelines/cmd/workspace"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
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
	root.AddCommand(newRobotLogin("robot-login"))

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

func newRobotLogin(use string) *cobra.Command {
	robotLogin := &cobra.Command{
		Use:    use,
		Short:  "Login as a robot.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			accessToken, err := tui.AskSecret(ctx, "Which Access Token would you like to use today?", "That would be a Github access token.", "access token")
			if err != nil {
				return err
			}

			username, err := fnapi.LoginAsRobotAndStore(ctx, args[0], string(accessToken))
			if err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "\nHi %s, you are now logged in, have a nice day.\n", username)
			return nil
		}),
	}

	return robotLogin
}
