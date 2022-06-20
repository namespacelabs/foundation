// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/source"
)

func newBufGenerateCmd() *cobra.Command {
	lang := "go"

	cmd := &cobra.Command{
		Use:     "proto-generate",
		Short:   "Run buf.build generate on your codebase.",
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

			var fmwk source.OpProtoGen_Framework
			switch lang {
			case "go":
				fmwk = source.OpProtoGen_GO
			case "typescript":
				fmwk = source.OpProtoGen_TYPESCRIPT
			default:
				return fmt.Errorf("unsupported language: %s", lang)
			}

			return source.GenProtosAtPaths(ctx, env, root.FS(), source.ProtosOpts{Framework: fmwk}, clean, root.FS())
		}),
	}

	cmd.Flags().StringVar(&lang, "lang", lang,
		"Language for proto generation. Supported values: go, typescript.")

	return cmd
}
