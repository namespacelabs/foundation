// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build !windows

package fncobra

import "github.com/spf13/cobra"

// MarkAsNotSupportedOnWindows is a no-op on non-Windows platforms.
func MarkAsNotSupportedOnWindows(cmd *cobra.Command) {}
