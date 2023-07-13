// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gcp

import "github.com/spf13/cobra"

func NewGcpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcp",
		Short: "Google Cloud Platform Federation related commands.",
	}

	cmd.AddCommand(newImpersonateCmd())

	return cmd
}
