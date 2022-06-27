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
	"namespacelabs.dev/foundation/internal/fnfs"
)

const (
	goServicePkg   = "api/echoservice"
	goServiceName  = "EchoService"
	goServerPkg    = "server/api"
	goServerName   = "ApiServer"
	webServicePkg  = "web/ui"
	webServiceName = "WebService"
	webServerPkg   = "server/web"
	webServerName  = "WebServer"
	readmeFilePath = "README.md"
	readmeTemplate = `Your starter Namespace project has been generated!

Next steps:

- Switch to the project directory: ` + "`" + `cd %s` + "`" + `
- Run ` + "`" + `ns prepare local` + "`" + ` to prepare the local dev environment.
- Run ` + "`" + `ns dev %s` + "`" + ` to start the server stack in the development mode with hot reload.
`
)

func newStarterCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
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
				description: fmt.Sprintf("Adding an example Go API service '%s' at '%s'.", goServiceName, goServicePkg),
				args: []string{"create", "service", goServicePkg,
					"--framework=go",
					fmt.Sprintf("--name=%s", goServiceName)},
			},
			{
				description: fmt.Sprintf("Adding an example Go API server '%s' at '%s'.", goServerName, goServerPkg),
				args: []string{"create", "server", goServerPkg,
					"--framework=go",
					fmt.Sprintf("--name=%s", goServerName),
					fmt.Sprintf("--service=%s/%s", workspaceName, goServicePkg),
				},
			},
			{
				description: fmt.Sprintf("Adding an example Web service '%s' at '%s'.", webServiceName, webServicePkg),
				args: []string{"create", "service", webServicePkg,
					"--framework=web",
					fmt.Sprintf("--http_backend=%s/%s", workspaceName, goServerPkg),
				},
			},
			{
				description: fmt.Sprintf("Adding an example Web server '%s' at '%s'.", webServerName, webServerPkg),
				args: []string{"create", "server", webServerPkg, "--framework=web",
					fmt.Sprintf("--name=%s", webServerName),
					fmt.Sprintf("--http_service=/:%s/%s", workspaceName, webServicePkg)},
			},
			{
				description: "Bringing language-specific configuration up to date, making it consistent with the Namespace configuration. Downloading language-specific dependencies.",
				args:        []string{"tidy"},
			},
		}

		for _, starterCmd := range starterCmds {
			printConsoleCmd(ctx, stdout, fmt.Sprintf("ns %s", strings.Join(starterCmd.args, " ")))
			fmt.Fprintf(stdout, "%s\n\n", colors.Ctx(ctx).Comment.Apply(starterCmd.description))
			if err := runCommand(ctx, starterCmd.args); err != nil {
				return err
			}
		}

		readmeContent := fmt.Sprintf(readmeTemplate, dirName, webServerPkg)
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := writeFileIfDoesntExist(ctx, fnfs.ReadWriteLocalFS(cwd), readmeFilePath, readmeContent); err != nil {
			return err
		}

		fmt.Fprintf(stdout, "\n\n%s\n", colors.Ctx(ctx).Highlight.Apply(readmeContent))

		return nil
	})

	return cmd
}

type starterCmd struct {
	description string
	args        []string
}

func printConsoleCmd(ctx context.Context, out io.Writer, text string) {
	fmt.Fprintf(out, "\n> %s\n", colors.Ctx(ctx).Highlight.Apply(text))
}
