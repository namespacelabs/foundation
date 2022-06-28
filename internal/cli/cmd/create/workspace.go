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
	vscodeExtensionsFilePath = ".vscode/extensions.json"
	vscodeExtensionsTemplate = `{
    "recommendations": [
        "golang.go",
        "esbenp.prettier-vscode",
        "zxh404.vscode-proto3",
        "namespacelabs.namespace-vscode"
    ]
}`
	gitignoreFilePath = ".gitignore"
	gitignoreTemplate = `# Namespace configuration of this specific host.
devhost.textpb

# Typescript/Node.js/Yarn
node_modules
**/.yarn/*
!**/.yarn/patches
!**/.yarn/plugins
!**/.yarn/releases
!**/.yarn/sdks
!**/.yarn/versions
`
)

func newWorkspaceCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
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

		if err := writeWorkspaceConfig(ctx, fsfs, args); err != nil {
			return err
		}
		if err := writeFileIfDoesntExist(ctx, console.Stdout(ctx), fsfs, vscodeExtensionsFilePath, vscodeExtensionsTemplate); err != nil {
			return err
		}
		if err := writeFileIfDoesntExist(ctx, console.Stdout(ctx), fsfs, gitignoreFilePath, gitignoreTemplate); err != nil {
			return err
		}

		return runCommand(ctx, []string{"tidy"})
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
		if workspaceName == "" {
			return "", context.Canceled
		}
		return workspaceName, nil
	} else {
		return args[0], nil
	}
}

func writeWorkspaceConfig(ctx context.Context, fsfs fnfs.ReadWriteFS, args []string) error {
	workspaceName, err := workspaceNameFromArgs(ctx, args)
	if err != nil {
		return err
	}
	if workspaceName == "" {
		return context.Canceled
	}

	return writeFileIfDoesntExist(ctx, nil, fsfs, cuefrontend.WorkspaceFile, fmt.Sprintf(workspaceFileTemplate, workspaceName, versions.APIVersion))
}

func writeFileIfDoesntExist(ctx context.Context, out io.Writer, fsfs fnfs.ReadWriteFS, fn string, content string) error {
	f, err := fsfs.Open(fn)
	if err == nil {
		f.Close()
		return nil
	}

	return fnfs.WriteWorkspaceFile(ctx, out, fsfs, fn, func(w io.Writer) error {
		_, err := fmt.Fprint(w, content)
		return err
	})
}
