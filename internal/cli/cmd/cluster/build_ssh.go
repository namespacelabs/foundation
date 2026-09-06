// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/public"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
	builderv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/builder/v1beta"
)

func newBuildSshCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ssh <build-ref> [command...]",
		Short: "Connect to the builder that is executing a build.",
		Long:  "Connect to the builder that is executing a build. On remote builders, `buildctl` is on the `PATH` and points at the local `buildkitd`.",
		Example: `nsc build ssh <build-ref>                     # interactive
nsc build ssh <build-ref> -- <command...>     # non-interactive`,
		Args:   cobra.MinimumNArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		record, err := getBuildRecord(ctx, args[0])
		if err != nil {
			return err
		}

		instanceID := record.GetBuildMachine().GetInstanceId()
		if instanceID == "" {
			return fnerrors.Newf("build %q has no builder instance", args[0])
		}

		cluster, err := api.GetCluster(tasks.WithSink(ctx, tasks.NullSink()), api.Methods, instanceID)
		if err != nil {
			return err
		}
		if cluster.Cluster == nil || cluster.Cluster.DestroyedAt != "" {
			return fnerrors.Newf("builder instance for build %q is not running", args[0])
		}
		ssh := api.ClusterService(cluster.Cluster, "ssh")
		if ssh == nil || ssh.Status != "READY" {
			return fnerrors.Newf("builder instance for build %q is not available for SSH", args[0])
		}
		return InlineSsh(ctx, cluster.Cluster, InlineSshOpts{}, args[1:])
	})

	return cmd
}

func getBuildRecord(ctx context.Context, buildRef string) (*builderv1beta.BuildRecord, error) {
	token, err := fnapi.IssueBearerToken(ctx)
	if err != nil {
		return nil, err
	}

	client, conn, err := public.NewBuilderServiceClient(ctx, ids.NewRandomBase32ID(4), token)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	record, err := client.GetBuildRecord(ctx, &builderv1beta.GetBuildRecordRequest{BuildRef: buildRef})
	if err != nil {
		switch status.Code(err) {
		case codes.Unavailable, codes.Unknown:
			return nil, fnerrors.Newf("unable to resolve build %q", buildRef)
		}
		return nil, err
	}
	if record.GetBuildRef() == "" {
		return nil, fnerrors.Newf("build %q not found", buildRef)
	}
	return record, nil
}
