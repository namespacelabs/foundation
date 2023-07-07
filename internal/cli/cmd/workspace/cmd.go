// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package workspace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func NewWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Interact with Namespace workspace.",
	}

	cmd.AddCommand(newDescribeCmd())

	return cmd
}

func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe current workspace details.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		res, err := fnapi.GetTenant(ctx)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)
		switch *output {
		case "json":
			d := json.NewEncoder(stdout)
			d.SetIndent("", "  ")
			return d.Encode(res.Tenant)

		default:
			fmt.Fprintf(stdout, "\nWorkspace details:\n\n")
			fmt.Fprintf(stdout, "Name: %s\n", res.Tenant.Name)
			fmt.Fprintf(stdout, "Tenant ID: %s\n", res.Tenant.TenantId)
		}

		return nil
	})

	return cmd
}
