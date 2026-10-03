// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/compute"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
	networkv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/network/v1beta"
)

var newEgressPolicyClient = fnapi.NewEgressPolicyServiceClient

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
		client, err := newEgressPolicyClient(ctx)
		if err != nil {
			return fnerrors.Newf("failed to create egress policy client: %w", err)
		}

		res, err := client.ListEgressPolicies(ctx, connect.NewRequest(&networkv1beta.ListEgressPoliciesRequest{}))
		if err != nil {
			return fnerrors.Newf("failed to list egress policies: %w", err)
		}

		return printEgressPolicies(ctx, *output, res.Msg.Policies)
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
		client, err := newEgressPolicyClient(ctx)
		if err != nil {
			return fnerrors.Newf("failed to create egress policy client: %w", err)
		}

		res, err := client.GetEgressPolicy(ctx, connect.NewRequest(&networkv1beta.GetEgressPolicyRequest{Tag: args[0]}))
		if err != nil {
			return fnerrors.Newf("failed to get egress policy %q: %w", args[0], err)
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			if err := rejectUnknownEgressPolicyFields(res.Msg.Policy); err != nil {
				return err
			}
			pretty, err := marshalEgressPolicy(res.Msg.Policy)
			if err != nil {
				return fnerrors.InternalError("failed to encode egress policy: %w", err)
			}
			fmt.Fprintln(stdout, string(pretty))
			return nil
		}
		if *output != "plain" {
			return fnerrors.Newf("invalid output format: %s", *output)
		}

		return printEgressPolicyDescription(stdout, res.Msg.Policy, res.Msg.Revision)
	})
}

func printEgressPolicyDescription(output io.Writer, policy *networkv1beta.EgressPolicy, revision int64) error {
	w := tabwriter.NewWriter(output, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "Tag:\t%s\n", policy.GetTag())
	fmt.Fprintf(w, "Description:\t%s\n", policy.GetDescription())
	fmt.Fprintf(w, "Mode:\t%s\n", policy.GetSpec().GetMode())
	fmt.Fprintf(w, "Deep packet inspection:\t%t\n", policy.GetSpec().GetDeepPacketInspection())
	fmt.Fprintf(w, "Rules:\t%d\n", len(policy.GetSpec().GetRules()))
	fmt.Fprintf(w, "Revision:\t%d\n", revision)
	return w.Flush()
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

		policy, err := parseEgressPolicy(contents)
		if err != nil {
			return err
		}

		client, err := newEgressPolicyClient(ctx)
		if err != nil {
			return fnerrors.Newf("failed to create egress policy client: %w", err)
		}

		if _, err := client.CreateEgressPolicy(ctx, connect.NewRequest(&networkv1beta.CreateEgressPolicyRequest{Policy: policy})); err != nil {
			return fnerrors.Newf("failed to create egress policy: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Created egress policy %q.\n", policy.Tag)
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

		client, err := newEgressPolicyClient(ctx)
		if err != nil {
			return fnerrors.Newf("failed to create egress policy client: %w", err)
		}

		current, err := client.GetEgressPolicy(ctx, connect.NewRequest(&networkv1beta.GetEgressPolicyRequest{Tag: args[0]}))
		if err != nil {
			return fnerrors.Newf("failed to get egress policy %q: %w", args[0], err)
		}
		if err := rejectUnknownEgressPolicyFields(current.Msg.Policy); err != nil {
			return err
		}

		if _, err := client.UpdateEgressPolicy(ctx, connect.NewRequest(&networkv1beta.UpdateEgressPolicyRequest{
			Policy:        policy,
			MatchRevision: current.Msg.Revision,
		})); err != nil {
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

func parseEgressPolicy(contents []byte) (*networkv1beta.EgressPolicy, error) {
	contents, err := normalizeEgressPolicyJSON(contents)
	if err != nil {
		return nil, err
	}

	policy := &networkv1beta.EgressPolicy{}
	if err := protojson.Unmarshal(contents, policy); err != nil {
		return nil, fnerrors.Newf("invalid egress policy configuration: %w", err)
	}
	if strings.TrimSpace(policy.Tag) == "" {
		return nil, fnerrors.New("invalid egress policy configuration: tag is required")
	}

	return policy, nil
}

func parseEgressPolicyUpdate(contents []byte, tag string) (*networkv1beta.EgressPolicy, error) {
	contents, err := normalizeEgressPolicyJSON(contents)
	if err != nil {
		return nil, err
	}

	policy := &networkv1beta.EgressPolicy{}
	if err := protojson.Unmarshal(contents, policy); err != nil {
		return nil, fnerrors.Newf("invalid egress policy configuration: %w", err)
	}
	if policy.Tag != "" && policy.Tag != tag {
		return nil, fnerrors.Newf("egress policy tag %q in --spec_file does not match requested tag %q", policy.Tag, tag)
	}
	policy.Tag = tag

	return policy, nil
}

func normalizeEgressPolicyJSON(contents []byte) ([]byte, error) {
	var policy map[string]json.RawMessage
	if err := json.Unmarshal(contents, &policy); err != nil {
		return nil, fnerrors.Newf("invalid egress policy configuration: %w", err)
	}
	if policy == nil {
		return nil, fnerrors.New("invalid egress policy configuration: expected a JSON object")
	}
	if _, hasSpec := policy["spec"]; hasSpec {
		return contents, nil
	}

	spec := map[string]json.RawMessage{}
	fields := (&networkv1beta.EgressPolicySpec{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		field := fields.Get(i)
		for _, name := range []string{string(field.Name()), field.JSONName()} {
			if value, ok := policy[name]; ok {
				spec[name] = value
				delete(policy, name)
			}
		}
	}
	if len(spec) == 0 {
		return contents, nil
	}

	encodedSpec, err := json.Marshal(spec)
	if err != nil {
		return nil, fnerrors.InternalError("failed to encode egress policy spec: %w", err)
	}
	policy["spec"] = encodedSpec

	encodedPolicy, err := json.Marshal(policy)
	if err != nil {
		return nil, fnerrors.InternalError("failed to encode egress policy: %w", err)
	}
	return encodedPolicy, nil
}

func marshalEgressPolicy(policy *networkv1beta.EgressPolicy) ([]byte, error) {
	if err := rejectUnknownEgressPolicyFields(policy); err != nil {
		return nil, err
	}

	metadata, err := json.Marshal(struct {
		Tag         string `json:"tag"`
		Description string `json:"description,omitempty"`
	}{
		Tag:         policy.Tag,
		Description: policy.Description,
	})
	if err != nil {
		return nil, fnerrors.InternalError("failed to encode egress policy metadata: %w", err)
	}

	encodedSpec := []byte("{}")
	if policy.Spec != nil {
		encodedSpec, err = (protojson.MarshalOptions{UseProtoNames: true}).Marshal(policy.Spec)
		if err != nil {
			return nil, fnerrors.InternalError("failed to encode egress policy spec: %w", err)
		}
	}

	flattened := bytes.NewBuffer(make([]byte, 0, len(metadata)+len(encodedSpec)))
	flattened.Write(metadata[:len(metadata)-1])
	if len(encodedSpec) > 2 {
		flattened.WriteByte(',')
		flattened.Write(encodedSpec[1 : len(encodedSpec)-1])
	}
	flattened.WriteByte('}')

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, flattened.Bytes(), "", "  "); err != nil {
		return nil, fnerrors.InternalError("failed to format flattened egress policy: %w", err)
	}
	return pretty.Bytes(), nil
}

func rejectUnknownEgressPolicyFields(policy *networkv1beta.EgressPolicy) error {
	if policy != nil && messageHasUnknownFields(policy.ProtoReflect()) {
		return fnerrors.New("egress policy contains fields unknown to this nsc version; upgrade nsc before describing or updating it")
	}
	return nil
}

func messageHasUnknownFields(message protoreflect.Message) bool {
	if len(message.GetUnknown()) != 0 {
		return true
	}

	found := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsMap() {
			if field.MapValue().Kind() == protoreflect.MessageKind {
				value.Map().Range(func(_ protoreflect.MapKey, value protoreflect.Value) bool {
					found = messageHasUnknownFields(value.Message())
					return !found
				})
			}
			return !found
		}
		if field.IsList() {
			if field.Kind() == protoreflect.MessageKind {
				list := value.List()
				for i := 0; i < list.Len() && !found; i++ {
					found = messageHasUnknownFields(list.Get(i).Message())
				}
			}
			return !found
		}
		if field.Kind() == protoreflect.MessageKind {
			found = messageHasUnknownFields(value.Message())
		}
		return !found
	})
	return found
}

func printEgressPolicies(ctx context.Context, output string, entries []*networkv1beta.ListEgressPoliciesResponse_EgressPolicyEntry) error {
	if output == "json" {
		views := make([]egressPolicyView, 0, len(entries))
		for _, entry := range entries {
			views = append(views, egressPolicyView{Tag: entry.Policy.Tag, Description: entry.Policy.Description})
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
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "No egress policies configured.")
		return nil
	}

	for _, entry := range entries {
		if entry.Policy.Description == "" {
			fmt.Fprintln(stdout, entry.Policy.Tag)
		} else {
			fmt.Fprintf(stdout, "%s\t%s\n", entry.Policy.Tag, entry.Policy.Description)
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

		token, err := fnapi.FetchToken(ctx)
		if err != nil {
			return fnerrors.Newf("Authentication error: %w", err)
		}

		cli, err := compute.NewClient(ctx, token)
		if err != nil {
			return fnerrors.Newf("Connection error %w", err)
		}

		timestampRange, err := fncobra.ParseTimestampRange(before, after)
		if err != nil {
			return err
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
