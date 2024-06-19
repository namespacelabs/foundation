// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingress",
		Short: "Ingress-related activities.",
	}

	cmd.AddCommand(newListIngressesCmd())
	cmd.AddCommand(newGenerateAccessTokenCmd())

	return cmd
}

type ingressOut struct {
	Port int32  `json:"port,omitempty"`
	Fqdn string `json:"fqdn,omitempty"`
}

func newListIngressesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists the registered ingresses on the specified instance.",
		Args:  cobra.MaximumNArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		lst, err := api.ListIngresses(ctx, api.Methods, cluster)
		if err != nil {
			return err
		}

		switch *output {
		case "plain":
			for _, ingress := range lst.ExportedInstancePort {
				parts := []string{fmt.Sprintf("port: %d", ingress.Port)}
				if ingress.Description != "" {
					parts = append(parts, ingress.Description)
				}

				fmt.Fprintf(console.Stdout(ctx), "https://%s (%s)\n", ingress.IngressFqdn, strings.Join(parts, "; "))
			}

		case "json":
			var res []ingressOut
			for _, ingress := range lst.ExportedInstancePort {
				res = append(res, ingressOut{
					Port: ingress.Port,
					Fqdn: ingress.IngressFqdn,
				})
			}

			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			if err := enc.Encode(res); err != nil {
				return fnerrors.InternalError("failed to encode instance as JSON output: %w", err)
			}
		}

		return nil
	})

	return cmd
}

func newGenerateAccessTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-access-token",
		Short: "Generate a Namespace Cloud token to access a preview for the current workspace.",
		Args:  cobra.NoArgs,
	}

	instance := cmd.Flags().String("instance", "", "Limit the access token to this instance.")
	outputPath := cmd.Flags().String("output_to", "", "If specified, write the access token to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		res, err := fnapi.IssueIngressAccessToken(ctx, *instance)
		if err != nil {
			return err
		}

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(res.IngressAccessToken), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputPath, err)
			}

			return nil
		}

		fmt.Fprintln(console.Stdout(ctx), res.IngressAccessToken)
		return nil
	})

	return cmd
}
