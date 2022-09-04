// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package sdk

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	golangsdk "namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/languages/golang"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func newGoCmd(goVersion string) *cobra.Command {
	var (
		env planning.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:                "go -- ...",
			Short:              "Run Go.",
			DisableFlagParsing: true,
		}).
		With(
			fncobra.FixedEnv(&env, "dev")).
		DoWithArgs(func(ctx context.Context, args []string) error {
			pl := workspace.NewPackageLoader(env)
			loc, err := pl.Resolve(ctx, schema.Name(env.Workspace().ModuleName))
			if err != nil {
				return err
			}

			sdk, err := golangsdk.MatchSDK(goVersion, golangsdk.HostPlatform())
			if err != nil {
				return fnerrors.Wrap(loc, err)
			}

			localSDK, err := compute.GetValue(ctx, sdk)
			if err != nil {
				return fnerrors.Wrap(loc, err)
			}

			return golang.RunGo(ctx, loc, localSDK, args...)
		})
}
