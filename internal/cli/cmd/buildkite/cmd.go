// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkite

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	buildkitepb "namespacelabs.dev/integrations/proto/namespace/cloud/buildkite"
	"namespacelabs.dev/integrations/proto/namespace/cloud/buildkite/buildkiteconnect"
	iamv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/iam/v1beta"
)

var newQueueServiceClient func(context.Context) (buildkiteconnect.QueueServiceClient, error) = fnapi.NewBuildkiteQueueServiceClient

func NewBuildkiteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "buildkite",
		Aliases: []string{"bk"},
		Short:   "Manage Buildkite resources.",
	}
	cmd.AddCommand(newQueuesCmd())
	return cmd
}

func newQueuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queues",
		Short: "Manage Namespace-managed Buildkite queues.",
	}
	cmd.AddCommand(newQueuesListCmd(), newQueuesGetCmd(), newQueuesUpdateCmd())
	return cmd
}

func newQueuesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Buildkite queues registered for the current workspace.",
		Args:  cobra.NoArgs,
	}
	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		client, err := newQueueServiceClient(ctx)
		if err != nil {
			return err
		}
		resp, err := client.ListQueues(ctx, connect.NewRequest(&buildkitepb.ListQueuesRequest{}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queues list", "failed to list queues: %w", err)
		}
		return printJSON(ctx, resp.Msg)
	})
}

func newQueuesGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <queue-id>",
		Short: "Get a Namespace-managed Buildkite queue.",
		Args:  cobra.ExactArgs(1),
	}
	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
		client, err := newQueueServiceClient(ctx)
		if err != nil {
			return err
		}
		resp, err := client.GetQueue(ctx, connect.NewRequest(&buildkitepb.GetQueueRequest{QueueId: args[0]}))
		if err != nil {
			return fnerrors.InvocationError("buildkite queues get", "failed to get queue: %w", err)
		}
		return printJSON(ctx, resp.Msg)
	})
}

func newQueuesUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <queue-id>",
		Short: "Update settings for a Namespace-managed Buildkite queue.",
		Args:  cobra.ExactArgs(1),
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

	return fncobra.Cmd(cmd).DoWithArgs(func(ctx context.Context, args []string) error {
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
				return fnerrors.InvocationError("buildkite queues update", "failed to update queue: %w", err)
			}
			return printJSON(ctx, resp.Msg)
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
			return fnerrors.InvocationError("buildkite queues update", "failed to get current queue settings: %w", err)
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
			return fnerrors.InvocationError("buildkite queues update", "failed to update queue: %w", err)
		}
		return printJSON(ctx, resp.Msg)
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

func printJSON(ctx context.Context, message proto.Message) error {
	formatted, err := protojson.MarshalOptions{Indent: "  "}.Marshal(message)
	if err != nil {
		return fnerrors.InternalError("failed to format response: %w", err)
	}
	_, err = fmt.Fprintln(console.Stdout(ctx), string(formatted))
	return err
}
