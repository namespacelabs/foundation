// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/tenants"
)

func NewExchangeGithubTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-github-token", // TODO find better name & group commands - hidden cmd for now.
		Short:  "Generate a Namspace Cloud token from a GitHub JWT.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		return tenants.RefreshTokenForGithubAction(ctx)
	})
}
