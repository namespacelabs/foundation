// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runs

import "github.com/spf13/cobra"

func NewRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "runs",
	}

	cmd.AddCommand(newNewCmd())
	cmd.AddCommand(newCompleteCmd())

	return cmd
}
