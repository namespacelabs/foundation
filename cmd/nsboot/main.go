// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/std/tasks"
)

func main() {
	fncobra.SetupViper()
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	sink, style, cleanup := fncobra.ConsoleToSink(fncobra.StandardConsole())
	ctxWithSink := colors.WithStyle(tasks.WithSink(context.Background(), sink), style)
	rootCtx := fnapi.WithTelemetry(ctxWithSink)

	// It's a bit awkward, but the main command execution is split between the command proper
	// and the execution of the inner ns binary after all the nsboot cleanup is done.
	// This variable is passes the package to execute from inside the command to the outside.
	var nsPackage nsboot.NSPackage

	rootCmd := &cobra.Command{
		Use:                "nsboot",
		Args:               cobra.ArbitraryArgs,
		SilenceUsage:       true,
		SilenceErrors:      true,
		DisableFlagParsing: true,

		RunE: func(cmd *cobra.Command, args []string) (err error) {
			nsPackage, err = nsboot.UpdateToRun(cmd.Context())
			return
		},
	}
	rootCmd.AddCommand(&cobra.Command{
		Use:   "update-ns",
		Short: "Checks and downloads updates for the ns command.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nsboot.ForceUpdate(cmd.Context())
		},
	})
	rootCmd.Flags().ParseErrorsWhitelist.UnknownFlags = true
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	err := compute.Do(rootCtx, func(ctx context.Context) (err error) {
		return rootCmd.ExecuteContext(ctx)
	})
	if cleanup != nil {
		cleanup()
	}
	if err != nil {
		format.Format(os.Stderr, err, format.WithStyle(style))
		return
	}

	// We make sure to flush all the output before starting the command.
	if nsPackage != "" {
		if err := nsPackage.Execute(context.Background()); err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				os.Exit(exiterr.ExitCode())
			} else {
				format.Format(os.Stderr, err, format.WithStyle(style))
				os.Exit(3)
			}
		}
	}
}
