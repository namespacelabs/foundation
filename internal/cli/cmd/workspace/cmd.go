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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
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
	jsonKey := cmd.Flags().StringP("key", "k", "", "Select a field to print if in json output mode.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *jsonKey != "" && *output != "json" {
			return fnerrors.Newf("--key requires --output=json")
		}

		res, err := fnapi.GetTenant(ctx)
		if err != nil {
			return err
		}

		imgReg, err := api.GetImageRegistry(ctx, api.Methods)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)
		switch *output {
		case "json":
			out := jsonOut{Tenant: res.Tenant}
			if nscr := imgReg.NSCR; nscr != nil {
				out.RegistryUrl = fmt.Sprintf("%s/%s", nscr.EndpointAddress, nscr.Repository)
			}

			if *jsonKey == "" {
				d := json.NewEncoder(stdout)
				d.SetIndent("", "  ")
				if err := d.Encode(out); err != nil {
					return fnerrors.InternalError("failed to encode tenant as JSON output: %w", err)
				}

				return nil
			}

			data, err := json.Marshal(out)
			if err != nil {
				return fnerrors.InternalError("failed to encode tenant as JSON output: %w", err)
			}

			// XXX All selectable keys are strings for now.
			// Parsing into string to make it obvious if this assumption ever breaks.
			parsed := map[string]string{}
			if err := json.Unmarshal(data, &parsed); err != nil {
				return fnerrors.InternalError("failed to decode JSON: %w", err)
			}

			selected, ok := parsed[*jsonKey]
			if !ok {
				return fnerrors.Newf("selected json key %q not found in response", *jsonKey)
			}

			// As all selectable values are strings, we do not JSON marshal here, to keep
			// the output easy to consume programatically (e.g. no quotation of plain strings).
			fmt.Fprintf(stdout, "%v\n", selected)

		default:
			fmt.Fprintf(stdout, "\nWorkspace details:\n\n")
			fmt.Fprintf(stdout, "Name: %s\n", res.Tenant.Name)
			fmt.Fprintf(stdout, "Tenant ID: %s\n", res.Tenant.TenantId)

			if nscr := imgReg.NSCR; nscr != nil {
				fmt.Fprintf(stdout, "Registry URL: %s/%s\n", nscr.EndpointAddress, nscr.Repository)
			}
		}

		return nil
	})

	return cmd
}

type jsonOut struct {
	*fnapi.Tenant

	RegistryUrl string `json:"registry_url,omitempty"`
}
