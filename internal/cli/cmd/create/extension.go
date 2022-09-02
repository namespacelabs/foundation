// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/frontend/cue"
	"namespacelabs.dev/foundation/provision"
)

func newExtensionCmd() *cobra.Command {
	var (
		targetPkg targetPkg
		env       provision.Env
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "extension [path/to/package]",
			Short: "Creates an extension.",
		}).
		With(fncobra.FixedEnv(&env, "dev")).
		With(parseTargetPkgWithDeps(&targetPkg, "extension")...).
		Do(func(ctx context.Context) error {
			if err := cue.CreateExtensionScaffold(ctx, targetPkg.Root.FS(), targetPkg.Loc); err != nil {
				return err
			}

			return codegenNode(ctx, env, targetPkg.Root, targetPkg.Loc)
		})
}
