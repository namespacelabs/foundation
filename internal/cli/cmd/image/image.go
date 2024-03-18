// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package image

import "github.com/spf13/cobra"

func NewImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "image",
		Short:  "Internal commands, for debugging.",
		Hidden: true,
	}

	cmd.AddCommand(newMakeDiskCmd())

	return cmd
}
