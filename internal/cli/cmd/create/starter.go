// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
)

const (
	starterServicePkg  = "api/echoservice"
	starterServiceName = "EchoService"
	starterServerPkg   = "server"
	starterServerName  = "Server"
)

func newStarterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "starter",
		Short: "Creates a new workspace in a new directory from a template.",
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		workspaceName, err := workspaceNameFromArgs(ctx, args)
		if err != nil || workspaceName == "" {
			return err
		}

		nameParts := strings.Split(workspaceName, "/")
		dirName := nameParts[len(nameParts)-1]
		if err := os.MkdirAll(dirName, 0755); err != nil {
			return err
		}
		if err := os.Chdir(dirName); err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		printConsoleCmd(ctx, stdout, fmt.Sprintf("mkdir %s; cd %s", dirName, dirName))

		starterCmds := []starterCmd{
			{
				description: "Bootstrapping the workspace configuration.",
				args:        []string{"create", "workspace", workspaceName},
			},
			{
				description: fmt.Sprintf("Adding an example service %s.", starterServiceName),
				args:        []string{"create", "service", starterServicePkg, "--framework=go", fmt.Sprintf("--name=%s", starterServiceName)},
			},
			{
				description: fmt.Sprintf("Adding an example server %s.", starterServerName),
				args:        []string{"create", "server", starterServerPkg, "--framework=go", fmt.Sprintf("--name=%s", starterServerName)},
			},
		}

		rootCmd := cmd.Root()
		for _, starterCmd := range starterCmds {
			if err := runAndPrintCommand(ctx, stdout, rootCmd, &starterCmd); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}

type starterCmd struct {
	description string
	args        []string
}

func runAndPrintCommand(ctx context.Context, out io.Writer, rootCmd *cobra.Command, starterCmd *starterCmd) error {
	printConsoleCmd(ctx, out, fmt.Sprintf("ns %s", strings.Join(starterCmd.args, " ")))
	fmt.Fprintf(out, "%s\n\n", colors.Ctx(ctx).Comment.Apply(starterCmd.description))
	return runCommand(ctx, rootCmd, starterCmd.args)
}

func printConsoleCmd(ctx context.Context, out io.Writer, text string) {
	fmt.Fprintf(out, "\n> %s\n", colors.Ctx(ctx).Highlight.Apply(text))
}
