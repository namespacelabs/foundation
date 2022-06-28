// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

const (
	goServicePkg   = "api/echoservice"
	goServiceName  = "EchoService"
	goServerPkg    = "server/api"
	goServerName   = "apiserver"
	webServicePkg  = "web/ui"
	webServiceName = "WebService"
	webServerPkg   = "server/web"
	webServerName  = "webserver"
	readmeFilePath = "README.md"
)

var (
	readmeTemplate = template.Must(template.New("readme").Parse(`Your starter Namespace project has been generated!

Next steps:

{{if .Dir -}}
- Switch to the project directory: ` + "`" + `cd {{.Dir}}` + "`" + `
{{end -}}
- Run ` + "`" + `ns prepare local` + "`" + ` to prepare the local dev environment.
- Run ` + "`" + `ns dev {{.ServerPkg}}` + "`" + ` to start the server stack in the development mode with hot reload.
`))
)

type readmeTmplOpts struct {
	Dir       string
	ServerPkg string
}

func newStarterCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "starter [directory]",
		Short: "Creates a new workspace from a template.",
		Long:  "Creates a new workspace from a template (Go server + Web server) in the given directory.",
		Args:  cobra.MaximumNArgs(1),
	}

	workspaceName := cmd.Flags().String("workspace_name", "", "Name of the workspace.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		stdout := console.Stdout(ctx)

		fmt.Fprintf(stdout, "\nSeting up a starter project with an api server in Go and a web frontend. It will take a few minutes.\n")

		var err error
		var dirName string

		if *workspaceName == "" {
			*workspaceName, err = askWorkspaceName(ctx)
			if err != nil {
				return err
			}
			if *workspaceName == "" {
				return context.Canceled
			}
		}

		if len(args) > 0 {
			dirName = args[0]
		} else {
			nameParts := strings.Split(*workspaceName, "/")
			dirNamePlaceholder := nameParts[len(nameParts)-1]
			dirName, err = tui.Ask(ctx,
				"Directory for the new project?",
				"It can be a relative or an absolute path. Use '.' to generate the project in the current directory.",
				dirNamePlaceholder)
			if err != nil {
				return err
			}
		}

		if dirName != "." {
			if err := os.MkdirAll(dirName, 0755); err != nil {
				return err
			}

			fmt.Fprintf(stdout, "\nCreated directory '%s'.\n", dirName)

			if err := os.Chdir(dirName); err != nil {
				return err
			}

			printConsoleCmd(ctx, stdout, fmt.Sprintf("cd %s", dirName))
		}

		starterCmds := []starterCmd{
			{
				description: "Bootstrapping the workspace configuration.",
				args:        []string{"create", "workspace", *workspaceName},
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
					fmt.Sprintf("--with_service=%s/%s", *workspaceName, goServicePkg),
				},
			},
			{
				description: fmt.Sprintf("Adding an example Web service '%s' at '%s'.", webServiceName, webServicePkg),
				args: []string{"create", "service", webServicePkg,
					"--framework=web",
					fmt.Sprintf("--with_http_backend=%s/%s", *workspaceName, goServerPkg),
				},
			},
			{
				description: fmt.Sprintf("Adding an example Web server '%s' at '%s'.", webServerName, webServerPkg),
				args: []string{"create", "server", webServerPkg, "--framework=web",
					fmt.Sprintf("--name=%s", webServerName),
					fmt.Sprintf("--with_http_service=/:%s/%s", *workspaceName, webServicePkg)},
			},
			{
				description: "Bringing the language-specific configuration up to date, making it consistent with the Namespace configuration. Downloading language-specific dependencies.\nIt may take a few minutes.",
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

		return generateAndPrintReadme(ctx, stdout, dirName)
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

func generateAndPrintReadme(ctx context.Context, out io.Writer, dir string) error {
	// No need to change the directorty if it's the current one.
	if dir == "." {
		dir = ""
	}
	data := readmeTmplOpts{
		Dir:       dir,
		ServerPkg: webServerPkg,
	}

	var body bytes.Buffer
	if err := readmeTemplate.Execute(&body, data); err != nil {
		return fnerrors.InternalError("failed to apply template: %w", err)
	}

	readmeContent := body.String()

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := writeFileIfDoesntExist(ctx, nil, fnfs.ReadWriteLocalFS(cwd), readmeFilePath, readmeContent); err != nil {
		return err
	}

	fmt.Fprintf(out, "\n\n%s\n", colors.Ctx(ctx).Highlight.Apply(readmeContent))

	return nil
}
