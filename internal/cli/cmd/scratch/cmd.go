// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package scratch

import (
	"github.com/spf13/cobra"
)

func NewScratchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scratch",
		Short: "Manage temporary scratch instances.",
	}

	cmd.AddCommand(NewSshCmd())

	return cmd
}
