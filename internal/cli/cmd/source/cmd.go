// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package source

import (
	"github.com/spf13/cobra"
)

func NewSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "source",
		Short:   "Source-related commands.",
		Aliases: []string{"src"},
	}

	cmd.AddCommand(newBufGenerateCmd())
	cmd.AddCommand(newNewIdCmd())

	return cmd
}
