// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/grpcurl"
)

func newGRPCurlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "grpcurl",
		Short:              "Run grpcurl.",
		DisableFlagParsing: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bin, err := grpcurl.EnsureSDK(ctx)
			if err != nil {
				return err
			}

			return localexec.RunInteractive(ctx, exec.CommandContext(ctx, string(bin), args...))
		}),
	}

	return cmd
}
