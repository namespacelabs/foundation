// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/integrations/api/compute"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
)

func NewWorkspaceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace",
		Short: "Interact with Namespace workspace.",
	}

	cmd.AddCommand(newDescribeCmd())
	cmd.AddCommand(newConcurrencyCmd())

	return cmd
}

func newConcurrencyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "concurrency",
		Short: "Display current workspace resource usage.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		resp, err := getConcurrency(ctx)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)
		switch *output {
		case "json":
			body, err := protojson.MarshalOptions{Indent: "  "}.Marshal(resp)
			if err != nil {
				return fnerrors.InternalError("failed to encode concurrency as JSON output: %w", err)
			}
			fmt.Fprintln(stdout, string(body))
		case "plain":
			bar := progress.New(progress.WithSolidFill("#1c32ff"), progress.WithWidth(24), progress.WithoutPercentage())
			title := lipgloss.NewStyle().Bold(true)
			detail := lipgloss.NewStyle().Faint(true)
			for _, entry := range resp.GetConcurrency() {
				active := entry.GetActiveConcurrency()
				limits := entry.GetLimits()
				fmt.Fprintf(stdout, "%s  %s\n", title.Render(entry.GetName()), detail.Render(strings.Join(entry.GetPlatforms(), ", ")))
				if limits.GetMaxInstanceCount() > 0 {
					printConcurrencyBar(stdout, bar, "Instances", active.GetInstanceCount(), limits.GetMaxInstanceCount(), fmt.Sprintf("%d / %d", active.GetInstanceCount(), limits.GetMaxInstanceCount()))
				}
				if limits.GetMaxCpu() > 0 {
					printConcurrencyBar(stdout, bar, "vCPU", active.GetCpu(), limits.GetMaxCpu(), fmt.Sprintf("%d / %d", active.GetCpu(), limits.GetMaxCpu()))
				}
				if limits.GetMaxMemoryMb() > 0 {
					printConcurrencyBar(stdout, bar, "Memory", active.GetMemoryMb(), limits.GetMaxMemoryMb(), fmt.Sprintf("%s / %s",
						humanize.IBytes(uint64(active.GetMemoryMb())*humanize.MiByte),
						humanize.IBytes(uint64(limits.GetMaxMemoryMb())*humanize.MiByte),
					))
				}
				fmt.Fprintln(stdout)
			}
		default:
			return fnerrors.Newf("unknown output format %q; expected plain or json", *output)
		}

		return nil
	})

	return cmd
}

func printConcurrencyBar(stdout io.Writer, bar progress.Model, label string, active, limit int64, value string) {
	fmt.Fprintf(stdout, "  %-9s %s  %s\n", label, bar.ViewAs(float64(active)/float64(limit)), value)
}

func getConcurrency(ctx context.Context) (*computev1beta.GetConcurrencyResponse, error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, fnerrors.Newf("authentication error: %w", err)
	}

	client, err := compute.NewClient(ctx, token)
	if err != nil {
		return nil, fnerrors.Newf("connection error: %w", err)
	}
	defer client.Close()

	resp, err := client.Usage.GetConcurrency(ctx, &computev1beta.GetConcurrencyRequest{})
	if err != nil {
		return nil, fnerrors.Newf("failed to get workspace concurrency: %w", err)
	}

	return resp, nil
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
