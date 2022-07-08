// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/module"
)

func newNodejsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Run nodejs.",
	}

	var relPath string

	yarn := fncobra.CmdWithEnv(&cobra.Command{
		Use:   "yarn",
		Short: "Run Yarn.",
	}, func(ctx context.Context, env provision.Env, args []string) error {
		root, err := module.FindRoot(ctx, ".")
		if err != nil {
			return err
		}

		if relPath == "" {
			var err error
			relPath, err = relCwd(ctx)
			if err != nil {
				return err
			}
		}

		return nodejs.RunYarn(ctx, env, relPath, args, root.WorkspaceData)
	})

	yarn.Flags().StringVar(&relPath, "rel_path", "", "If not set, will be computed.")

	cmd.AddCommand(yarn)

	return fncobra.CmdWithEnv(cmd, func(ctx context.Context, env provision.Env, args []string) error {
		relPath, err := relCwd(ctx)
		if err != nil {
			return err
		}

		return nodejs.RunNodejs(ctx, env, relPath, "node", &nodejs.RunNodejsOpts{Args: args, IsInteractive: true})
	})
}

func relCwd(ctx context.Context) (string, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return "", err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Rel(root.Abs(), cwd)
}
