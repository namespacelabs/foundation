// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"context"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/sdk/grpcurl"
	"namespacelabs.dev/foundation/internal/sdk/host"
)

func newGRPCurlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "grpcurl -- ...",
		Short:              "Run grpcurl.",
		DisableFlagParsing: true,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			bin, err := grpcurl.EnsureSDK(ctx, host.HostPlatform())
			if err != nil {
				return err
			}

			return localexec.RunInteractive(ctx, exec.CommandContext(ctx, string(bin), args...))
		}),
	}

	return cmd
}
