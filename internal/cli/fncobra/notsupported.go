// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import "github.com/spf13/cobra"

// unsupportedOnWindowsAnnotation marks a command that does not yet work on
// Windows. It is only ever set on Windows (see MarkAsNotSupportedOnWindows);
// commands carrying it, directly or via a parent, abort early in the root
// PersistentPreRunE.
const unsupportedOnWindowsAnnotation = "ns.unsupported-on-windows"

// notSupportedOnWindowsMessage is shown when a command flagged as unsupported on
// Windows is invoked.
const notSupportedOnWindowsMessage = "This command is not yet supported on Windows."

// isUnsupportedOnWindows reports whether the command, or any of its ancestors,
// has been marked as unsupported on Windows.
func isUnsupportedOnWindows(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if c.Annotations[unsupportedOnWindowsAnnotation] == "true" {
			return true
		}
	}

	return false
}
