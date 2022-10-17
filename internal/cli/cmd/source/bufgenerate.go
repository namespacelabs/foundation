// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	source "namespacelabs.dev/foundation/internal/codegen"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newBufGenerateCmd() *cobra.Command {
	var (
		lang string
		env  cfg.Context
		locs fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:     "proto-generate [--lang go|typescript] <path>...",
			Short:   "Run buf.build generate on your codebase.",
			Aliases: []string{"proto-gen", "protogen"},
			Args:    cobra.MinimumNArgs(1),
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&lang, "lang", "go", "Language for proto generation. Supported values: go, typescript.")
		}).
		With(
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env)).
		Do(func(ctx context.Context) error {
			var paths []string
			for _, loc := range locs.Locs {
				if loc.ModuleName != env.Workspace().ModuleName() {
					return fnerrors.InternalError("%s: can't run codegen on files outside of the current workspace", loc.ModuleName)
				}

				paths = append(paths, loc.RelPath)
			}

			loc, err := parsing.NewPackageLoader(env).Resolve(ctx, schema.PackageName(env.Workspace().ModuleName()))
			if err != nil {
				return err
			}

			var fmwk schema.Framework
			switch lang {
			case "go":
				fmwk = schema.Framework_GO
			case "typescript":
				fmwk = schema.Framework_NODEJS
			default:
				return fmt.Errorf("unsupported language: %s", lang)
			}

			if err := source.GenProtosAtPaths(ctx, env, fmwk, loc.Module.ReadOnlyFS(), paths, loc.Module.ReadWriteFS()); err != nil {
				return err
			}

			return nil
		})
}
