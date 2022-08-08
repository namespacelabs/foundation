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

	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/git"
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
	testPkg        = "tests/echo"
	readmeFilePath = "README.md"
)

func newStarterCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	var (
		workspaceName  string
		dryRun         bool
		suggestPrepare bool
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "starter [directory]",
			Short: "Creates a new workspace from a template.",
			Long:  "Creates a new workspace from a template (Go server + Web server) in the given directory.",
			Args:  cobra.MaximumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&workspaceName, "workspace_name", "", "Name of the workspace.")
			flags.BoolVar(&dryRun, "dry_run", false, "If true, does not create the workspace and only prints the commands.")
			_ = flags.MarkHidden("dry_run")
			flags.BoolVar(&suggestPrepare, "suggest_prepare", true, "If true, suggest to run `ns prepare` in README. This is false when `prepare` has been done already, e.g. in a Gitpod instance.")
			_ = flags.MarkHidden("suggest_prepare")
		}).
		DoWithArgs(func(ctx context.Context, args []string) error {
			stdout := console.Stdout(ctx)

			fmt.Fprintf(stdout, "\nSetting up a starter project with an api server in Go and a web frontend. It will take a few minutes.\n")

			var err error
			var dirName string

			if workspaceName == "" {
				// Trying to auto-detect a git repository.
				if isRoot, err := git.IsRepoRoot(ctx); err == nil && isRoot {
					if url, err := git.RemoteUrl(ctx); err == nil {
						workspaceName = url
						if len(args) == 0 {
							dirName = "."
						}
					}
				} else {
					workspaceName, err = askWorkspaceName(ctx)
					if err != nil {
						return err
					}
					if workspaceName == "" {
						return context.Canceled
					}
				}
			}

			if dirName == "" {
				if len(args) > 0 {
					dirName = args[0]
				} else {
					nameParts := strings.Split(workspaceName, "/")
					dirNamePlaceholder := nameParts[len(nameParts)-1]
					dirName, err = tui.Ask(ctx,
						"Directory for the new project?",
						"It can be a relative or an absolute path. Use '.' to generate the project in the current directory.",
						dirNamePlaceholder)
					if err != nil {
						return err
					}
				}
			}

			if dirName != "." {
				if !dryRun {
					if err := os.MkdirAll(dirName, 0755); err != nil {
						return err
					}
				}

				fmt.Fprintf(stdout, "\nCreated directory '%s'.\n", dirName)

				if !dryRun {
					if err := os.Chdir(dirName); err != nil {
						return err
					}
				}

				printConsoleCmd(ctx, stdout, fmt.Sprintf("cd %s", dirName))
			}

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
						fmt.Sprintf("--with_service=%s/%s", workspaceName, goServicePkg),
					},
				},
				{
					description: fmt.Sprintf("Adding an example Web service '%s' at '%s'.", webServiceName, webServicePkg),
					args: []string{"create", "service", webServicePkg,
						"--framework=web",
						fmt.Sprintf("--with_http_backend=%s/%s", workspaceName, goServerPkg),
					},
				},
				{
					description: fmt.Sprintf("Adding an example Web server '%s' at '%s'.", webServerName, webServerPkg),
					args: []string{"create", "server", webServerPkg, "--framework=web",
						fmt.Sprintf("--name=%s", webServerName),
						fmt.Sprintf("--with_http_service=/:%s/%s", workspaceName, webServicePkg)},
				},
				{
					description: "Adding an example e2e test.",
					args: []string{"create", "test", testPkg,
						fmt.Sprintf("--server=%s/%s", workspaceName, goServerPkg),
						fmt.Sprintf("--service=%s/%s", workspaceName, goServicePkg)},
				},
				{
					description: "Bringing the language-specific configuration up to date, making it consistent with the Namespace configuration. Downloading language-specific dependencies.\nIt may take a few minutes.",
					args:        []string{"tidy"},
				},
			}

			for _, starterCmd := range starterCmds {
				printConsoleCmd(ctx, stdout, fmt.Sprintf("ns %s", strings.Join(starterCmd.args, " ")))
				fmt.Fprintf(stdout, "%s\n", wordwrap.String(colors.Ctx(ctx).Comment.Apply(starterCmd.description), 80))

				if !dryRun {
					fmt.Fprintln(stdout)
					if err := runCommand(ctx, starterCmd.args); err != nil {
						return err
					}
				}
			}

			// README.md file content and the content to print to console are slightly different.
			if !dryRun {
				err = generateAndWriteReadmeFile(ctx, stdout, suggestPrepare)
				if err != nil {
					return err
				}
			}
			return generateAndPrintStarterInfo(ctx, stdout, &starterInfoOpts{
				Dir:            dirName,
				DryRun:         dryRun,
				SuggestPrepare: suggestPrepare,
			})
		})
}

type starterCmd struct {
	description string
	args        []string
}

func printConsoleCmd(ctx context.Context, out io.Writer, text string) {
	fmt.Fprintf(out, "\n> %s\n", colors.Ctx(ctx).Highlight.Apply(text))
}

func generateAndWriteReadmeFile(ctx context.Context, out io.Writer, suggestPrepare bool) error {
	opts := starterInfoOpts{
		SuggestPrepare: suggestPrepare,
	}
	if isRoot, err := git.IsRepoRoot(ctx); err == nil && isRoot {
		if url, err := git.RemoteUrl(ctx); err == nil {
			opts.RepoUrlForGitpod = fmt.Sprintf("https://%s", url)
		}
	}

	readmeContent, err := generateStarterInfo(ctx, readmeFileTemplate, &opts)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := writeFileIfDoesntExist(ctx, nil, fnfs.ReadWriteLocalFS(cwd), readmeFilePath, readmeContent); err != nil {
		return err
	}

	return nil
}

func generateAndPrintStarterInfo(ctx context.Context, out io.Writer, opts *starterInfoOpts) error {
	readmeContent, err := generateStarterInfo(ctx, starterInfoTemplate, opts)
	if err != nil {
		return err
	}

	fmt.Fprintln(out)
	return renderMarkdown(out, readmeContent)
}
