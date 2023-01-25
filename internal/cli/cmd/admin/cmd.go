// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package admin

import (
	"context"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Admin-related activities (internal only).",
	}

	cmd.AddCommand(newBlockTenantCmd())
	cmd.AddCommand(newUnblockTenantCmd())

	return cmd
}

func newBlockTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "block-tenant",
		Short: "Blocks a tenant.",
		Args:  cobra.ExactArgs(1),
	}

	service := cmd.Flags().String("service", "vm", "Which service to block the tenant for. Valid options: 'vm' (default), 'api'.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.New("expected exactly one tenant id, got %d", len(args))
		}

		tenantId := args[0]

		switch *service {
		case "vm":
			return api.BlockTenant(ctx, api.Endpoint, tenantId)
		case "api":
			return fnapi.BlockTenant(ctx, tenantId)
		default:
			return fnerrors.New("invalid service %q", *service)
		}
	})

	return cmd
}

func newUnblockTenantCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unblock-tenant",
		Short: "Unblocks a tenant.",
		Args:  cobra.ExactArgs(1),
	}
	service := cmd.Flags().String("service", "vm", "Which service to unblock the tenant for. Valid options: 'vm' (default), 'api'.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if len(args) != 1 {
			return fnerrors.New("expected exactly one tenant id, got %d", len(args))
		}

		tenantId := args[0]

		switch *service {
		case "vm":
			return api.UnblockTenant(ctx, api.Endpoint, tenantId)
		case "api":
			return fnapi.UnblockTenant(ctx, tenantId)
		default:
			return fnerrors.New("invalid service %q", *service)
		}
	})

	return cmd
}
