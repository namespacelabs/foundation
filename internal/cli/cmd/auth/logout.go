// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/console"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func NewLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout from Namespace",
		Args:  cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			if err := auth.DeleteStoredToken(); err != nil {
				return err
			}

			if err := cluster.DeleteSessionClientCert(); err != nil {
				return err
			}

			fmt.Fprintf(console.Stdout(ctx), "\nYou are now logged out, have a nice day.\n")
			return nil
		}),
	}

	return cmd
}
