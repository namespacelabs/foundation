// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
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

			done := console.EnterInputMode(ctx)
			defer done()

			kubectl := exec.CommandContext(ctx, string(bin), args...)
			kubectl.Stdout = os.Stdout
			kubectl.Stderr = os.Stderr
			kubectl.Stdin = os.Stdin
			return kubectl.Run()
		}),
	}

	return cmd
}
