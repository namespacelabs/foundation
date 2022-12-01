// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package mod

import (
	"context"

	"github.com/spf13/cobra"
)

func NewModCmd(runCommand func(context.Context, []string) error) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mod",
		Short: "Module related operations (e.g. init, download, get, tidy).",
	}

	cmd.AddCommand(NewTidyCmd())
	cmd.AddCommand(newInitCmd(runCommand))
	cmd.AddCommand(newDownloadCmd())
	cmd.AddCommand(newGetCmd())

	return cmd
}
