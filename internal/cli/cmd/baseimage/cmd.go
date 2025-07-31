// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package baseimage

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster/github"
)

func NewBaseImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "base-image",
		Short: "Operations for managing your base images.",
	}

	cmd.AddCommand(newUploadBaseImageCmd())
	cmd.AddCommand(github.NewBaseImageBuildCmd("build-github-image"))

	return cmd
}
