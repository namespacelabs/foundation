// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewIngressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ingress",
		Short: "Ingress-related activities.",
	}

	cmd.AddCommand(newListIngressesCmd())

	return cmd
}

func newListIngressesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Lists the registered ingresses on the specified instance.",
		Args:  cobra.MaximumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		lst, err := api.ListIngresses(ctx, api.Methods, cluster.ClusterId)
		if err != nil {
			return err
		}

		for _, ingress := range lst.ExportedInstancePort {
			parts := []string{fmt.Sprintf("port: %d", ingress.Port)}
			if ingress.Description != "" {
				parts = append(parts, ingress.Description)
			}
			fmt.Fprintf(console.Stdout(ctx), "https://%s (%s)\n", ingress.IngressFqdn, strings.Join(parts, "; "))
		}

		return nil
	})

	return cmd
}
