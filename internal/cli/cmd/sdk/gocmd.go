// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package sdk

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	golangsdk "namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/languages/golang"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
)

func newGoCmd(goVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "go -- ...",
		Short:              "Run Go.",
		DisableFlagParsing: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			pl := workspace.NewPackageLoader(root, nil /* env */)
			loc, err := pl.Resolve(ctx, schema.Name(root.Workspace().ModuleName))
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
		}),
	}

	return cmd
}
