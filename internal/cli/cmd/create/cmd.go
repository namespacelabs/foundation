// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package create

import (
	"context"

	"github.com/spf13/cobra"
)

func NewCreateCmd(runCommand func(ctx context.Context, args []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Creates a new extension, service, server or workspace (also from a starter template).",
		Aliases: []string{"c"},
		Hidden:  true,
	}

	cmd.AddCommand(newExtensionCmd())
	cmd.AddCommand(newServerCmd(runCommand))
	cmd.AddCommand(newServiceCmd(runCommand))
	cmd.AddCommand(newWorkspaceCmd(runCommand))
	cmd.AddCommand(newStarterCmd(runCommand))
	cmd.AddCommand(newTestCmd())

	return cmd
}
