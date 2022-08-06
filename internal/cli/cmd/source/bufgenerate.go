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
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source"
)

func newBufGenerateCmd() *cobra.Command {
	var (
		lang string
		env  provision.Env
		locs fncobra.Locations
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:     "proto-generate [--lang go|typescript] <path>...",
			Short:   "Run buf.build generate on your codebase.",
			Aliases: []string{"proto-gen", "protogen"},
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&lang, "lang", "go", "Language for proto generation. Supported values: go, typescript.")
		}).
		With(
			fncobra.FixedEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &fncobra.ParseLocationsOpts{})).
		Do(func(ctx context.Context) error {
			paths := []string{}
			for _, loc := range locs.Locs {
				paths = append(paths, loc.RelPath)
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

			return source.GenProtosAtPaths(ctx, env, env.Root(), fmwk, paths, env.Root().FS())
		})
}
