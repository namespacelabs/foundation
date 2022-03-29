// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/source"
)

func newBufGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "proto-generate",
		// TODO: add a CLI flag to specify the languages when needed.
		Short:   "Run buf.build generate on your codebase. Only generates Go proto codegen.",
		Aliases: []string{"proto-gen", "protogen"},

		RunE: fncobra.RunE(func(ctx context.Context, userArgs []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			env, err := provision.RequireEnv(root, "dev")
			if err != nil {
				return err
			}

			clean := make([]string, len(userArgs))
			for k, str := range userArgs {
				clean[k] = filepath.Clean(str)
			}

			if len(clean) == 0 {
				clean = []string{"."}
			}

			return source.GenGoProtosAtPaths(ctx, env, workspace.NewPackageLoader(root), root.FS(),
				source.GoProtosOpts{Framework: source.OpProtoGen_GO}, clean, root.FS())
		}),
	}

	return cmd
}