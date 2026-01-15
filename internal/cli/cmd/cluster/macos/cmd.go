// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package macos

import "github.com/spf13/cobra"

func NewMacOSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "macos",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	cmd.AddCommand(newPackageCmd())

	return cmd
}
