// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

var getTenantIntegrations = fnapi.GetTenantIntegrations

func NewIntegrationsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "integrations",
		Short: "Manage workspace integrations.",
	}

	cmd.AddCommand(newTailscaleCmd())

	return cmd
}

func newTailscaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tailscale",
		Short: "Manage Tailscale integration specs for the current workspace.",
	}

	cmd.AddCommand(newTailscaleListCmd())
	cmd.AddCommand(newTailscaleSetCmd())
	cmd.AddCommand(newTailscaleRemoveCmd())

	return cmd
}

func newTailscaleListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List named Tailscale integration specs.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		res, err := getTenantIntegrations(ctx)
		if err != nil {
			return err
		}

		return printTailscaleList(ctx, *output, res)
	})
}

func newTailscaleSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Create or update a named Tailscale integration spec.",
		Args:  cobra.ExactArgs(1),
	}

	oauthClientID := cmd.Flags().String("oauth-client-id", "", "Tailscale OAuth client ID to attach to the named spec.")
	tags := cmd.Flags().StringSlice("tag", nil, "Tailscale tags to add to the named spec. Can be specified multiple times or as a comma-separated list.")
	enableMagicDNS := cmd.Flags().Bool("enable-magic-dns", false, "Enable Tailscale MagicDNS for the named spec.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		if *oauthClientID == "" {
			return fnerrors.Newf("--oauth-client-id is required")
		}

		name := args[0]
		res, err := updateTailscaleIntegrations(ctx, func(tailscale map[string]fnapi.TailscaleSpec) error {
			tailscale[name] = fnapi.TailscaleSpec{
				OauthClientId:  *oauthClientID,
				Tags:           append([]string(nil), (*tags)...),
				EnableMagicDNS: enableMagicDNS,
			}

			return nil
		})
		if err != nil {
			return err
		}

		return printTailscaleUpdateResult(ctx, *output, fmt.Sprintf("Updated Tailscale integration %q.\n", name), res)
	})
}

func newTailscaleRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a named Tailscale integration spec.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		name := args[0]
		res, err := updateTailscaleIntegrations(ctx, func(tailscale map[string]fnapi.TailscaleSpec) error {
			if _, ok := tailscale[name]; !ok {
				return fnerrors.Newf("tailscale integration %q not found", name)
			}

			delete(tailscale, name)
			return nil
		})
		if err != nil {
			return err
		}

		return printTailscaleUpdateResult(ctx, *output, fmt.Sprintf("Removed Tailscale integration %q.\n", name), res)
	})
}

func updateTailscaleIntegrations(ctx context.Context, mutate func(map[string]fnapi.TailscaleSpec) error) (fnapi.GetTenantIntegrationsResponse, error) {
	current, err := getTenantIntegrations(ctx)
	if err != nil {
		return fnapi.GetTenantIntegrationsResponse{}, err
	}

	tailscale := make(map[string]fnapi.TailscaleSpec, len(current.Tailscale))
	for name, spec := range current.Tailscale {
		tailscale[name] = spec
	}

	if err := mutate(tailscale); err != nil {
		return fnapi.GetTenantIntegrationsResponse{}, err
	}

	return fnapi.UpdateTenantIntegrations(ctx, current.MetadataVersion, tailscale)
}

func printTailscaleUpdateResult(ctx context.Context, output, plainMessage string, res fnapi.GetTenantIntegrationsResponse) error {
	if output == "json" {
		enc := json.NewEncoder(console.Stdout(ctx))
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fnerrors.InternalError("failed to encode integrations as JSON output: %w", err)
		}

		return nil
	}

	if output != "plain" {
		return fnerrors.Newf("invalid output format: %s", output)
	}

	fmt.Fprint(console.Stdout(ctx), plainMessage)
	return nil
}

func printTailscaleList(ctx context.Context, output string, res fnapi.GetTenantIntegrationsResponse) error {
	if output == "json" {
		enc := json.NewEncoder(console.Stdout(ctx))
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			return fnerrors.InternalError("failed to encode integrations as JSON output: %w", err)
		}

		return nil
	}

	if output != "plain" {
		return fnerrors.Newf("invalid output format: %s", output)
	}

	stdout := console.Stdout(ctx)
	if len(res.Tailscale) == 0 {
		fmt.Fprintln(stdout, "No Tailscale integrations configured.")
		return nil
	}

	names := make([]string, 0, len(res.Tailscale))
	for name := range res.Tailscale {
		names = append(names, name)
	}
	sort.Strings(names)

	fmt.Fprintln(stdout, "Tailscale integrations:")
	fmt.Fprintln(stdout)
	for _, name := range names {
		spec := res.Tailscale[name]
		fmt.Fprintln(stdout, name)
		fmt.Fprintf(stdout, "  OAuth Client ID: %s\n", spec.OauthClientId)
		if len(spec.Tags) == 0 {
			fmt.Fprintln(stdout, "  Tags: none")
		} else {
			fmt.Fprintf(stdout, "  Tags: %s\n", strings.Join(spec.Tags, ", "))
		}
		if spec.EnableMagicDNS != nil {
			if *spec.EnableMagicDNS {
				fmt.Fprintln(stdout, "  MagicDNS: Enabled")
			} else {
				fmt.Fprintln(stdout, "  MagicDNS: Disabled")
			}
		}
		fmt.Fprintln(stdout)
	}

	return nil
}
