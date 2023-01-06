// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package sdk

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/sdk/gcloud"
)

func newGcloudCmd() *cobra.Command {
	return fncobra.Cmd(
		&cobra.Command{
			Use:                "gcloud -- ...",
			Short:              "Run gcloud.",
			Hidden:             true,
			DisableFlagParsing: true,
		}).
		DoWithArgs(func(ctx context.Context, args []string) error {
			stdout := console.TypedOutput(ctx, "gcloud", common.CatOutputTool)

			return gcloud.Run(ctx, rtypes.IO{
				Stdout: stdout,
				Stderr: stdout,
			}, args)
		})
}
