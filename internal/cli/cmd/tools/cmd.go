// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package tools

import (
	"github.com/spf13/cobra"
)

func NewToolsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tools",
		Short:   "Tool-related commands.",
		Aliases: []string{"t", "tool"},
	}

	cmd.AddCommand(newGRPCurlCmd())
	cmd.AddCommand(NewKubeCtlCmd(false))

	return cmd
}
