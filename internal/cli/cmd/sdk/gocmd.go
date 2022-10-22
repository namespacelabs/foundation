// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sdk

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/languages/golang"
	"namespacelabs.dev/foundation/internal/parsing"
	golangsdk "namespacelabs.dev/foundation/internal/sdk/golang"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func newGoCmd(goVersion string) *cobra.Command {
	var (
		env cfg.Context
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:                "go -- ...",
			Short:              "Run Go.",
			Hidden:             true,
			DisableFlagParsing: true,
		}).
		With(
			fncobra.HardcodeEnv(&env, "dev")).
		DoWithArgs(func(ctx context.Context, args []string) error {
			pl := parsing.NewPackageLoader(env)
			loc, err := pl.Resolve(ctx, schema.MakePackageName(env.Workspace().ModuleName()))
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
