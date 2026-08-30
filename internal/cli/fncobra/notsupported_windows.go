// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build windows

package fncobra

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// MarkAsNotSupportedOnWindows flags a command as not yet supported on Windows.
// The command (and its subcommands) is hidden from help output and aborts with
// a warning when invoked.
//
// Runnable commands abort via the root PersistentPreRunE (see isUnsupportedOnWindows).
// Parent commands (those with subcommands but no Run/RunE) are not "runnable",
// so cobra short-circuits them to their help output before PersistentPreRunE
// runs. We therefore replace their help output too, so that invoking the bare
// command (e.g. `nsc docker`) or passing `--help` reports the unsupported status
// and aborts. The help func is inherited by subcommands, covering the whole tree.
func MarkAsNotSupportedOnWindows(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}

	cmd.Annotations[unsupportedOnWindowsAnnotation] = "true"
	cmd.Hidden = true

	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.ErrOrStderr(), notSupportedOnWindowsMessage)
		os.Exit(1)
	})
}
