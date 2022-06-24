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
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Initializes a workspace.",
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
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

		// Not announcing "write" since `tidy` will do it.
		err = fnfs.WriteWorkspaceFile(ctx, nil, fsfs, cuefrontend.WorkspaceFile, func(w io.Writer) error {
			_, err := fmt.Fprintf(w, workspaceFileTemplate, workspaceName, versions.APIVersion)
			return err
		})
		if err != nil {
			return err
		}

		return runCommand(ctx, cmd.Root(), []string{"tidy"})
	})

	return cmd
}

func askWorkspaceName(ctx context.Context) (string, error) {
	return tui.Ask(ctx,
		"Workspace name?",
		"The workspace name should to match the Github repository name.",
		"github.com/username/reponame")
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
