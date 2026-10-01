// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkite

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	buildkitepb "namespacelabs.dev/integrations/proto/namespace/cloud/buildkite"
	"namespacelabs.dev/integrations/proto/namespace/cloud/buildkite/buildkiteconnect"
	iamv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/iam/v1beta"
	"namespacelabs.dev/integrations/proto/namespace/stdlib"
)

var newQueueServiceClient func(context.Context) (buildkiteconnect.QueueServiceClient, error) = fnapi.NewBuildkiteQueueServiceClient

func NewBuildkiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "buildkite",
		Aliases: []string{"bk"},
		Short:   "Manage Buildkite resources.",
	}
	cmd.AddCommand(newQueuesCmd())
	queues := newQueuesCmd()
	queues.Use = "queues"
	queues.Hidden = true
	cmd.AddCommand(queues)
	return cmd
}

func newQueuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage Namespace-managed Buildkite queues.",
	}
	cmd.AddCommand(newQueuesListCmd(), newQueuesDescribeCmd(), newQueuesUpdateCmd())
	get := newQueuesDescribeCmd()
	get.Use = "get <queue-id>"
	get.Hidden = true
	cmd.AddCommand(get)
	return cmd
}

func newQueuesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Buildkite queues registered for the current workspace.",
		Example: `  nsc buildkite queue list
  nsc buildkite queue list -o json`,
		Args: cobra.NoArgs,
	}
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *output != "plain" && *output != "json" {
			return fnerrors.BadInputError("invalid --output %q: must be one of plain or json", *output)
		}
		client, err := newQueueServiceClient(ctx)
		if err != nil {
			return err
		}
		resp, err := client.ListQueues(ctx, connect.NewRequest(&buildkitepb.ListQueuesRequest{}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queue list", "failed to list queues: %w", err)
		}
		if *output == "json" {
			return printJSON(ctx, resp.Msg)
		}
		return printQueueTable(ctx, resp.Msg.GetQueues())
	})
}

func newQueuesDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <queue-id>",
		Short: "Describe a Namespace-managed Buildkite queue.",
		Example: `  nsc buildkite queue describe <queue-id>
  nsc buildkite queue describe <queue-id> -o json
  nsc buildkite queue describe <queue-id> -o spec > spec.json
  nsc buildkite queue update <queue-id> --spec_file spec.json`,
		Args: cobra.ExactArgs(1),
	}
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain, json, or spec. spec prints the queue settings in the format accepted by 'update --spec_file'.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		if *output != "plain" && *output != "json" && *output != "spec" {
			return fnerrors.BadInputError("invalid --output %q: must be one of plain, json, or spec", *output)
		}
		client, err := newQueueServiceClient(ctx)
		if err != nil {
			return err
		}
		resp, err := client.GetQueue(ctx, connect.NewRequest(&buildkitepb.GetQueueRequest{QueueId: args[0]}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queue describe", "failed to get queue: %w", err)
		}
		switch *output {
		case "json":
			return printJSON(ctx, resp.Msg)
		case "spec":
			return printQueueSpec(ctx, resp.Msg.GetQueue().GetSettings())
		default:
			return printQueueDetails(ctx, "Queue Details", resp.Msg.GetQueue())
		}
	})
}

func newQueuesUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <queue-id>",
		Short: "Update settings for a Namespace-managed Buildkite queue.",
		Example: `  nsc buildkite queue update <queue-id> --egress_policy restricted
  nsc buildkite queue update <queue-id> --spec_file spec.json -o json`,
		Args: cobra.ExactArgs(1),
	}

	specFile := cmd.Flags().String("spec_file", "", "Path to JSON file containing the queue settings. When provided, individual flags are ignored.")
	workloadPermissions := cmd.Flags().StringArray("workload_permissions", nil, `Custom workload permission as JSON (repeatable): {"resource_type":"...","resource_id":"...","actions":["..."]}`)
	resetPermissions := cmd.Flags().Bool("reset_permissions", false, "Remove custom workload permissions from this queue.")
	egressPolicy := cmd.Flags().String("egress_policy", "", "Workspace egress policy tag to apply to jobs running for this queue.")
	removeEgressPolicy := cmd.Flags().Bool("remove_egress_policy", false, "Remove the workspace egress policy tag from this queue.")
	reset := cmd.Flags().Bool("reset", false, "Reset all queue settings.")
	cmd.MarkFlagsMutuallyExclusive("reset", "workload_permissions")
	cmd.MarkFlagsMutuallyExclusive("reset", "reset_permissions")
	cmd.MarkFlagsMutuallyExclusive("reset", "egress_policy")
	cmd.MarkFlagsMutuallyExclusive("reset", "remove_egress_policy")
	cmd.MarkFlagsMutuallyExclusive("workload_permissions", "reset_permissions")
	cmd.MarkFlagsMutuallyExclusive("egress_policy", "remove_egress_policy")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		if *output != "plain" && *output != "json" {
			return fnerrors.BadInputError("invalid --output %q: must be one of plain or json", *output)
		}
		printResult := func(resp *buildkitepb.QueueResponse) error {
			if *output == "json" {
				return printJSON(ctx, resp)
			}
			return printQueueDetails(ctx, "Queue updated successfully", resp.GetQueue())
		}

		if *specFile != "" {
			settings, err := readQueueSettingsSpecFile(*specFile)
			if err != nil {
				return err
			}
			client, err := newQueueServiceClient(ctx)
			if err != nil {
				return err
			}
			resp, err := client.UpdateQueue(ctx, connect.NewRequest(&buildkitepb.UpdateQueueRequest{QueueId: args[0], Settings: settings}))
			if err != nil {
				return fnerrors.InvocationError("buildkite queue update", "failed to update queue: %w", err)
			}
			return printResult(resp.Msg)
		}

		permissions, err := makeQueuePermissions(*workloadPermissions, *resetPermissions)
		if err != nil {
			return err
		}
		egressPolicyChanged := cmd.Flags().Changed("egress_policy")
		if egressPolicyChanged && *egressPolicy == "" {
			return fnerrors.BadInputError("--egress_policy must not be empty; use --remove_egress_policy to remove it")
		}
		if !*reset && permissions == nil && !egressPolicyChanged && !*removeEgressPolicy {
			return fnerrors.BadInputError("--workload_permissions, --reset_permissions, --egress_policy, --remove_egress_policy, or --reset is required")
		}
		client, err := newQueueServiceClient(ctx)
		if err != nil {
			return err
		}
		current, err := client.GetQueue(ctx, connect.NewRequest(&buildkitepb.GetQueueRequest{QueueId: args[0]}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queue update", "failed to get current queue settings: %w", err)
		}

		settings := &buildkitepb.QueueSettings{}
		if currentSettings := current.Msg.GetQueue().GetSettings(); currentSettings != nil {
			settings = proto.Clone(currentSettings).(*buildkitepb.QueueSettings)
		}
		if *reset {
			settings.Reset()
		} else {
			if permissions != nil {
				settings.Permissions = permissions
			}
			if egressPolicyChanged {
				settings.EgressPolicyTag = *egressPolicy
			} else if *removeEgressPolicy {
				settings.EgressPolicyTag = ""
			}
		}
		resp, err := client.UpdateQueue(ctx, connect.NewRequest(&buildkitepb.UpdateQueueRequest{QueueId: args[0], Settings: settings}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queue update", "failed to update queue: %w", err)
		}
		return printResult(resp.Msg)
	})
}

func readQueueSettingsSpecFile(path string) (*buildkitepb.QueueSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fnerrors.Newf("failed to read spec file: %w", err)
	}

	settings := &buildkitepb.QueueSettings{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(data, settings); err != nil {
		return nil, fnerrors.Newf("failed to parse spec file: %w", err)
	}

	return settings, nil
}

func makeQueuePermissions(workloadPermissions []string, reset bool) (*buildkitepb.Permissions, error) {
	if reset {
		return &buildkitepb.Permissions{PermissionsType: buildkitepb.PermissionsType_DEFAULT}, nil
	}
	if len(workloadPermissions) == 0 {
		return nil, nil
	}

	permissions := &buildkitepb.Permissions{PermissionsType: buildkitepb.PermissionsType_CUSTOM}
	for _, value := range workloadPermissions {
		permission := &iamv1beta.Permission{}
		if err := protojson.Unmarshal([]byte(value), permission); err != nil {
			return nil, fnerrors.BadInputError("failed to parse workload permission JSON %q: %w", value, err)
		}
		if permission.GetResourceType() == "" {
			return nil, fnerrors.BadInputError("workload permission %q: resource_type is required", value)
		}
		if len(permission.GetActions()) == 0 {
			return nil, fnerrors.BadInputError("workload permission %q: at least one action is required", value)
		}
		permissions.WorkloadPermissions = append(permissions.WorkloadPermissions, permission)
	}
	return permissions, nil
}

func printQueueTable(ctx context.Context, queues []*buildkitepb.BuildkiteQueue) error {
	if len(queues) == 0 {
		_, err := fmt.Fprintln(console.Stdout(ctx), "No queues found.")
		return err
	}

	cols := []tui.Column{
		{Key: "id", Title: "ID", MinWidth: 10, MaxWidth: 40},
		{Key: "name", Title: "Name", MinWidth: 10, MaxWidth: 30},
		{Key: "cluster", Title: "Cluster", MinWidth: 10, MaxWidth: 40},
		{Key: "organization", Title: "Organization", MinWidth: 12, MaxWidth: 30},
	}

	rows := []tui.Row{}
	for _, queue := range queues {
		rows = append(rows, tui.Row{
			"id":           queue.GetId(),
			"name":         orDash(queue.GetQueueName()),
			"cluster":      orDash(queue.GetCluster()),
			"organization": orDash(queue.GetOrganization().GetOrgSlug()),
		})
	}

	return tui.StaticTable(ctx, cols, rows)
}

func printQueueDetails(ctx context.Context, header string, queue *buildkitepb.BuildkiteQueue) error {
	stdout := console.Stdout(ctx)

	fmt.Fprintf(stdout, "\n%s:\n\n", header)
	fmt.Fprintf(stdout, "Queue ID:      %s\n", queue.GetId())
	fmt.Fprintf(stdout, "Name:          %s\n", orDash(queue.GetQueueName()))
	fmt.Fprintf(stdout, "Cluster:       %s\n", orDash(queue.GetCluster()))
	if org := queue.GetOrganization(); org != nil {
		fmt.Fprintf(stdout, "\nOrganization:\n")
		fmt.Fprintf(stdout, "  Slug:        %s\n", orDash(org.GetOrgSlug()))
		fmt.Fprintf(stdout, "  UUID:        %s\n", orDash(org.GetOrgUuid()))
	}

	settings := queue.GetSettings()
	permissions := settings.GetPermissions()
	hasPermissions := permissions.GetPermissionsType() != buildkitepb.PermissionsType_PERMISSIONS_TYPE_UNKNOWN
	if hasPermissions || settings.GetEgressPolicyTag() != "" {
		fmt.Fprintf(stdout, "\nSettings:\n")
		if hasPermissions {
			fmt.Fprintf(stdout, "  Permissions:   %s\n", permissions.GetPermissionsType())
		}
		if settings.GetEgressPolicyTag() != "" {
			fmt.Fprintf(stdout, "  Egress Policy: %s\n", settings.GetEgressPolicyTag())
		}
	}

	if permissions.GetPermissionsType() == buildkitepb.PermissionsType_CUSTOM {
		fmt.Fprintf(stdout, "\nWorkload Permissions:\n")
		if len(permissions.GetWorkloadPermissions()) == 0 {
			fmt.Fprintf(stdout, "  (none)\n")
		}
		for _, permission := range permissions.GetWorkloadPermissions() {
			resourceID := permission.GetResourceId()
			if resourceID == "" {
				resourceID = "*"
			}
			fmt.Fprintf(stdout, "  - Resource: %s %s\n", permission.GetResourceType(), resourceID)
			fmt.Fprintf(stdout, "    Actions:  %s\n", strings.Join(permission.GetActions(), ", "))
		}
	}

	if otel := settings.GetOpenTelemetrySettings(); otel != nil && proto.Size(otel) > 0 {
		fmt.Fprintf(stdout, "\nOpenTelemetry:\n")
		fmt.Fprintf(stdout, "  Endpoint:    %s\n", orDash(otel.GetOtlpEndpoint()))
		fmt.Fprintf(stdout, "  Protocol:    %s\n", formatOTLPProtocol(otel.GetOtlpProtocol()))
		if len(otel.GetHeaders()) > 0 {
			fmt.Fprintf(stdout, "  Headers:\n")
			for _, header := range otel.GetHeaders() {
				fmt.Fprintf(stdout, "    - %s: %s\n", header.GetName(), formatHeaderValue(header))
			}
		}
		if attrs := otel.GetResourceAttributes(); len(attrs) > 0 {
			fmt.Fprintf(stdout, "  Resource Attributes:\n")
			for _, key := range slices.Sorted(maps.Keys(attrs)) {
				fmt.Fprintf(stdout, "    - %s=%s\n", key, attrs[key])
			}
		}
	}

	fmt.Fprintf(stdout, "\n")
	return nil
}

func formatOTLPProtocol(protocol buildkitepb.OpenTelemetrySettings_OTLPProtocol) string {
	switch protocol {
	case buildkitepb.OpenTelemetrySettings_OTLP_PROTOCOL_UNSPECIFIED:
		return "GRPC (default)"
	case buildkitepb.OpenTelemetrySettings_OTLP_PROTOCOL_GRPC:
		return "GRPC"
	case buildkitepb.OpenTelemetrySettings_OTLP_PROTOCOL_HTTP_PROTOBUF:
		return "HTTP_PROTOBUF"
	default:
		return protocol.String()
	}
}

// formatHeaderValue describes where a header value comes from without printing
// static values, which frequently contain credentials. Use -o json to see them.
func formatHeaderValue(header *stdlib.HttpHeader) string {
	if secretID := header.GetValueFrom().GetFromSecretId(); secretID != "" {
		return "from secret " + secretID
	}
	if header.GetValue() != "" || header.GetValueFrom().GetStatic() != "" {
		return "(static value)"
	}
	return "-"
}

// printQueueSpec prints the queue settings in the format accepted by `update --spec_file`.
func printQueueSpec(ctx context.Context, settings *buildkitepb.QueueSettings) error {
	if settings == nil {
		settings = &buildkitepb.QueueSettings{}
	} else if permissions := settings.GetPermissions(); permissions != nil && proto.Size(permissions) == 0 {
		settings = proto.Clone(settings).(*buildkitepb.QueueSettings)
		settings.Permissions = nil
	}
	formatted, err := protojson.MarshalOptions{Indent: "  ", UseProtoNames: true}.Marshal(settings)
	if err != nil {
		return fnerrors.InternalError("failed to format queue spec: %w", err)
	}
	_, err = fmt.Fprintln(console.Stdout(ctx), string(formatted))
	return err
}

func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func printJSON(ctx context.Context, message proto.Message) error {
	formatted, err := protojson.MarshalOptions{Indent: "  "}.Marshal(message)
	if err != nil {
		return fnerrors.InternalError("failed to format response: %w", err)
	}
	_, err = fmt.Fprintln(console.Stdout(ctx), string(formatted))
	return err
}
