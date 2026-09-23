// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package windows

import "github.com/spf13/cobra"

func NewWindowsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "windows",
		Short:  "Windows-specific utilities.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	cmd.AddCommand(newPackageCmd())

	return cmd
}
