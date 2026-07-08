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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/compute"
	"namespacelabs.dev/integrations/auth"
)

func NewEgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "egress",
		Short: "Egress-related activities.",
	}

	cmd.AddCommand(newEgressLogsCmd())

	return cmd
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
