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
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/compute"
	"namespacelabs.dev/integrations/auth"
)

var (
	listEgressPolicies = fnapi.ListEgressPolicies
	updateEgressPolicy = fnapi.UpdateEgressPolicy
)

const exampleEgressPolicyJSON = `{
  "tag": "example-policy",
  "description": "Allow access to example.com",
  "mode": "BLOCK",
  "rules": [
    {
      "op": "ALLOW",
      "matcher": {
        "match_domains": ["example.com"]
      }
    }
  ]
}`

const exampleEgressPolicyHelp = "Contents of an example --spec_file:\n" + exampleEgressPolicyJSON

func NewEgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "egress",
		Short: "Egress-related activities.",
	}

	cmd.AddCommand(newEgressLogsCmd())
	cmd.AddCommand(newEgressPolicyCmd())

	return cmd
}

func newEgressPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Manage tenant egress policies.",
		Args:  cobra.NoArgs,
	}

	cmd.AddCommand(newEgressPolicyListCmd())
	cmd.AddCommand(newEgressPolicyDescribeCmd())
	cmd.AddCommand(newEgressPolicyCreateCmd())
	cmd.AddCommand(newEgressPolicyUpdateCmd())

	return cmd
}

func newEgressPolicyListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available egress policies.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		res, err := listEgressPolicies(ctx)
		if err != nil {
			return fnerrors.Newf("failed to list egress policies: %w", err)
		}

		return printEgressPolicies(ctx, *output, res.Policies)
	})
}

func newEgressPolicyDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <tag>",
		Short: "Describe a single egress policy.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		res, err := listEgressPolicies(ctx)
		if err != nil {
			return fnerrors.Newf("failed to list egress policies: %w", err)
		}

		index, err := findEgressPolicy(res.Policies, args[0])
		if err != nil {
			return err
		}
		if index < 0 {
			return fnerrors.Newf("egress policy %q not found", args[0])
		}

		policy := res.Policies[index]

		var view egressPolicyView
		if err := json.Unmarshal(policy, &view); err != nil {
			return fnerrors.InternalError("failed to decode egress policy: %w", err)
		}

		pretty, err := json.MarshalIndent(policy, "", "  ")
		if err != nil {
			return fnerrors.InternalError("failed to encode egress policy: %w", err)
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			fmt.Fprintln(stdout, string(pretty))
			return nil
		}
		if *output != "plain" {
			return fnerrors.Newf("invalid output format: %s", *output)
		}

		fmt.Fprintf(stdout, "Tag:\t%s\n", view.Tag)
		if view.Description != "" {
			fmt.Fprintf(stdout, "Description:\t%s\n", view.Description)
		}
		fmt.Fprintln(stdout)
		fmt.Fprintln(stdout, string(pretty))

		return nil
	})
}

func newEgressPolicyCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create an egress policy from a JSON configuration file.",
		Example: exampleEgressPolicyHelp,
		Args:    cobra.NoArgs,
	}

	specFile := cmd.Flags().String("spec_file", "", "Path to JSON file containing the egress policy configuration.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *specFile == "" {
			printEgressPolicyExample(ctx)
			return fnerrors.New("--spec_file is required")
		}

		contents, err := os.ReadFile(*specFile)
		if err != nil {
			return fnerrors.Newf("failed to read egress policy configuration: %w", err)
		}

		policy, view, err := parseEgressPolicy(contents)
		if err != nil {
			return err
		}

		current, err := listEgressPolicies(ctx)
		if err != nil {
			return fnerrors.Newf("failed to list existing egress policies: %w", err)
		}

		index, err := findEgressPolicy(current.Policies, view.Tag)
		if err != nil {
			return err
		}
		if index >= 0 {
			return fnerrors.Newf("egress policy %q already exists", view.Tag)
		}

		if _, err := updateEgressPolicy(ctx, current.MetadataVersion, policy); err != nil {
			return fnerrors.Newf("failed to create egress policy: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Created egress policy %q.\n", view.Tag)
		return nil
	})
}

func newEgressPolicyUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "update <tag>",
		Short:   "Update an egress policy from a JSON configuration file.",
		Example: exampleEgressPolicyHelp,
		Args:    cobra.ExactArgs(1),
	}

	specFile := cmd.Flags().String("spec_file", "", "Path to JSON file containing the egress policy configuration. The policy tag may be omitted from the file.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		if *specFile == "" {
			printEgressPolicyExample(ctx)
			return fnerrors.New("--spec_file is required")
		}

		contents, err := os.ReadFile(*specFile)
		if err != nil {
			return fnerrors.Newf("failed to read egress policy configuration: %w", err)
		}

		policy, err := parseEgressPolicyUpdate(contents, args[0])
		if err != nil {
			return err
		}

		current, err := listEgressPolicies(ctx)
		if err != nil {
			return fnerrors.Newf("failed to list existing egress policies: %w", err)
		}

		index, err := findEgressPolicy(current.Policies, args[0])
		if err != nil {
			return err
		}
		if index < 0 {
			return fnerrors.Newf("egress policy %q not found", args[0])
		}

		if _, err := updateEgressPolicy(ctx, current.MetadataVersion, policy); err != nil {
			return fnerrors.Newf("failed to update egress policy: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Updated egress policy %q.\n", args[0])
		return nil
	})
}

func printEgressPolicyExample(ctx context.Context) {
	stdout := console.Stdout(ctx)
	fmt.Fprintln(stdout, "\nExample policy configuration:")
	fmt.Fprintln(stdout, exampleEgressPolicyJSON)
}

type egressPolicyView struct {
	Tag         string `json:"tag"`
	Description string `json:"description,omitempty"`
}

func findEgressPolicy(policies []json.RawMessage, tag string) (int, error) {
	for index, policy := range policies {
		var view egressPolicyView
		if err := json.Unmarshal(policy, &view); err != nil {
			return -1, fnerrors.InternalError("failed to decode existing egress policy: %w", err)
		}
		if view.Tag == tag {
			return index, nil
		}
	}

	return -1, nil
}

func parseEgressPolicy(contents []byte) (json.RawMessage, egressPolicyView, error) {
	var policy json.RawMessage
	if err := json.Unmarshal(contents, &policy); err != nil {
		return nil, egressPolicyView{}, fnerrors.Newf("invalid egress policy JSON: %w", err)
	}

	var view egressPolicyView
	if err := json.Unmarshal(policy, &view); err != nil {
		return nil, egressPolicyView{}, fnerrors.Newf("invalid egress policy configuration: %w", err)
	}
	if strings.TrimSpace(view.Tag) == "" {
		return nil, egressPolicyView{}, fnerrors.New("invalid egress policy configuration: tag is required")
	}

	return policy, view, nil
}

func parseEgressPolicyUpdate(contents []byte, tag string) (json.RawMessage, error) {
	var policy map[string]json.RawMessage
	if err := json.Unmarshal(contents, &policy); err != nil {
		return nil, fnerrors.Newf("invalid egress policy JSON: %w", err)
	}
	if policy == nil {
		return nil, fnerrors.New("invalid egress policy configuration: expected a JSON object")
	}

	if rawTag, ok := policy["tag"]; ok {
		var specTag string
		if err := json.Unmarshal(rawTag, &specTag); err != nil {
			return nil, fnerrors.Newf("invalid egress policy configuration: tag must be a string: %w", err)
		}
		if specTag != tag {
			return nil, fnerrors.Newf("egress policy tag %q in --spec_file does not match requested tag %q", specTag, tag)
		}
	} else {
		encodedTag, _ := json.Marshal(tag)
		policy["tag"] = encodedTag
	}

	encoded, err := json.Marshal(policy)
	if err != nil {
		return nil, fnerrors.InternalError("failed to encode egress policy: %w", err)
	}

	return encoded, nil
}

func printEgressPolicies(ctx context.Context, output string, policies []json.RawMessage) error {
	if output == "json" {
		views := make([]egressPolicyView, 0, len(policies))
		for _, policy := range policies {
			var view egressPolicyView
			if err := json.Unmarshal(policy, &view); err != nil {
				return fnerrors.InternalError("failed to decode egress policy: %w", err)
			}
			views = append(views, view)
		}

		enc := json.NewEncoder(console.Stdout(ctx))
		enc.SetIndent("", "  ")
		if err := enc.Encode(views); err != nil {
			return fnerrors.InternalError("failed to encode egress policies as JSON: %w", err)
		}
		return nil
	}
	if output != "plain" {
		return fnerrors.Newf("invalid output format: %s", output)
	}

	stdout := console.Stdout(ctx)
	if len(policies) == 0 {
		fmt.Fprintln(stdout, "No egress policies configured.")
		return nil
	}

	for _, policy := range policies {
		var view egressPolicyView
		if err := json.Unmarshal(policy, &view); err != nil {
			return fnerrors.InternalError("failed to decode egress policy: %w", err)
		}
		if view.Description == "" {
			fmt.Fprintln(stdout, view.Tag)
		} else {
			fmt.Fprintf(stdout, "%s\t%s\n", view.Tag, view.Description)
		}
	}

	return nil
}

func newEgressLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [instance-id]",
		Short: "Print egress filtering decisions for an instance.",
		Args:  cobra.ExactArgs(1),
	}

	after := cmd.Flags().String("after", "", "Only show records after this timestamp (RFC3339, e.g. 2024-01-15T10:30:00Z).")
	before := cmd.Flags().String("before", "", "Only show records before this timestamp (RFC3339, e.g. 2024-01-15T12:00:00Z).")
	limit := cmd.Flags().Int32("limit", 20000, "Maximum number of egress records to return.")
	output := cmd.Flags().StringP("output", "o", "plain", "Output format. Supported values: plain, json (outputs one JSON object per line).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {

		if *output != "plain" && *output != "json" {
			return fnerrors.Newf("unsupported output format %q, supported values: plain, json", *output)
		}

		token, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.Newf("Authentication error: %w", err)
		}

		cli, err := compute.NewClient(ctx, token)
		if err != nil {
			return fnerrors.Newf("Connection error %w", err)
		}

		var timestampRange *stdlib.TimestampRange
		if *after != "" || *before != "" {
			timestampRange = &stdlib.TimestampRange{}
			if *after != "" {
				t, err := time.Parse(time.RFC3339, *after)
				if err != nil {
					return fnerrors.Newf("invalid --after timestamp: %w", err)
				}
				timestampRange.After = timestamppb.New(t.UTC())
			}
			if *before != "" {
				t, err := time.Parse(time.RFC3339, *before)
				if err != nil {
					return fnerrors.Newf("invalid --before timestamp: %w", err)
				}
				timestampRange.Before = timestamppb.New(t.UTC())
			}
		}

		var records []*computev1beta.EgressRecord
		var cursor []byte

		for {
			req := &computev1beta.FetchInstanceEgressRequest{
				InstanceId:       args[0],
				Limit:            *limit - int32(len(records)),
				TimestampRange:   timestampRange,
				PaginationCursor: cursor,
			}

			resp, err := cli.Observability.FetchInstanceEgress(ctx, req)
			if err != nil {
				return fnerrors.Newf("failed to fetch instance egress: %w", err)
			}

			records = append(records, resp.Records...)

			if int32(len(records)) >= *limit || len(resp.PaginationCursor) == 0 {
				break
			}

			cursor = resp.PaginationCursor
		}

		if int32(len(records)) > *limit {
			records = records[:*limit]
		}

		if *output == "json" {
			enc := json.NewEncoder(os.Stdout)
			for _, rec := range records {
				if err := enc.Encode(rec); err != nil {
					return err
				}
			}
			return nil
		}

		if len(records) == 0 {
			fmt.Fprintf(console.Stdout(ctx), "No egress records found.\n")
			return nil
		}

		out := console.Stdout(ctx)
		for _, rec := range records {
			fmt.Fprintf(out, "%s  %-5s  %s",
				rec.Timestamp.AsTime().Format(time.RFC3339),
				egressAction(rec.Action),
				rec.Domain,
			)

			if rec.RuleMatch != "" {
				fmt.Fprintf(out, "  rule=%s", rec.RuleMatch)
			}
			if len(rec.AnswerIps) > 0 {
				fmt.Fprintf(out, "  ips=%s", strings.Join(rec.AnswerIps, ","))
			}
			fmt.Fprintln(out)
		}
		return nil
	})

	return cmd
}

func egressAction(action computev1beta.EgressAction) string {
	s := strings.TrimPrefix(action.String(), "ACTION_")
	if s == "UNKNOWN" {
		return "-"
	}
	return s
}
