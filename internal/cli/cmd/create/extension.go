// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package create

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/frontend/scaffold"
	"namespacelabs.dev/foundation/std/planning"
)

func newExtensionCmd() *cobra.Command {
	var (
		targetPkg targetPkg
		env       planning.Context
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "extension [path/to/package]",
			Short: "Creates an extension.",
		}).
		With(fncobra.HardcodeEnv(&env, "dev")).
		With(parseTargetPkgWithDeps(&targetPkg, "extension")...).
		Do(func(ctx context.Context) error {
			if err := scaffold.CreateExtensionScaffold(ctx, targetPkg.Root.ReadWriteFS(), targetPkg.Location); err != nil {
				return err
			}

			return codegenNode(ctx, targetPkg.Root, env, targetPkg.Location)
		})
}
