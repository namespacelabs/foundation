// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
)

func main() {
	fncobra.SetupViper()
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	rootCtx, style, flushLogs := fncobra.SetupContext(context.Background())

	rootCmd := &cobra.Command{
		Use:                "nsboot",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) error {
			_, pkg, err := nsboot.CheckUpdate(cmd.Context(), false, "")
			if err == nil {
				// We make sure to flush all the output before starting the command.
				flushLogs()

				pkg.ExecuteAndForwardExitCode(context.Background(), style)
				// Never returns.
			}
			return err
		},
	}

	rootCmd.AddCommand(cmd.NewUpdateNSCmd())

	rootCmd.Flags().ParseErrorsWhitelist.UnknownFlags = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	err := fncobra.RunInContext(rootCtx, func(ctx context.Context) error {
		return rootCmd.ExecuteContext(ctx)
	}, nil)

	flushLogs()

	if err != nil {
		format.Format(os.Stderr, err, format.WithStyle(style))
	}
}
