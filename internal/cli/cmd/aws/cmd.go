// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package aws

import "github.com/spf13/cobra"

func NewAwsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "AWS Federation related commands.",
	}

	cmd.AddCommand(newAssumeRoleCmd())
	cmd.AddCommand(newSetupWebIdentity())

	return cmd
}
