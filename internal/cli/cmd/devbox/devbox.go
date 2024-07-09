// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"github.com/spf13/cobra"
)

func NewDevBoxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "devbox",
		Short:  "Actions for devbox support.",
		Hidden: true, // for now
	}

	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newDestroyCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newDescribeCommand())
	cmd.AddCommand(newEnsureCommand())
	cmd.AddCommand(newSshCommand())
	cmd.AddCommand(newVscodeCommand())

	return cmd
}
