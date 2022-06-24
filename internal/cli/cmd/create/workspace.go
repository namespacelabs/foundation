// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/versions"
)

const (
	workspaceFileTemplate = `module: "%s"
requirements: {
	api: %d
}
`
)

func newWorkspaceCmd() *cobra.Command {
	use := "workspace"
	cmd := &cobra.Command{
		Use:   use,
		Short: "Initializes a workspace.",

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			fsfs := fnfs.ReadWriteLocalFS(cwd)
			f, err := fsfs.Open(cuefrontend.WorkspaceFile)
			if err == nil {
				f.Close()
				fmt.Fprintf(console.Stdout(ctx), "'%s' already exists, skipping.\n", cuefrontend.WorkspaceFile)
				return nil
			}

			workspaceName, err := workspaceNameFromArgs(ctx, args)
			if err != nil || workspaceName == "" {
				return err
			}

			return fnfs.WriteWorkspaceFile(ctx, console.Stdout(ctx), fsfs, cuefrontend.WorkspaceFile, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, workspaceFileTemplate, workspaceName, versions.APIVersion)
				return err
			})
		}),
	}

	return cmd
}

func askWorkspaceName(ctx context.Context) (string, error) {
	return tui.Ask(ctx,
		"Workspace name?",
		"If you plan to use this workspace from another workspace, the workspace name needs to match the Github repository name. For example, 'github.com/username/reponame'.",
		"foobar")
}

func workspaceNameFromArgs(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		workspaceName, err := askWorkspaceName(ctx)
		if err != nil {
			return "", err
		}
		return workspaceName, nil
	} else {
		return args[0], nil
	}
}
