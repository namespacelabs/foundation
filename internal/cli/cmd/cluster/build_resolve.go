// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/compute"
	buildkitepb "namespacelabs.dev/integrations/proto/namespace/cloud/buildkite"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
	githubv1beta "namespacelabs.dev/integrations/proto/namespace/cloud/github/v1beta"
)

const buildRefAttachmentTypeURL = "namespacelabs.dev/internal/build/ref"

func newBuildResolveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve container build references by origin.",
		Args:  cobra.NoArgs,
	}

	githubJob := cmd.Flags().Int64("github_job", 0, "Resolve container builds from a GitHub Actions job ID.")
	buildkiteJob := cmd.Flags().String("buildkite_job", "", "Resolve container builds from a Buildkite job ID.")
	originInstance := cmd.Flags().String("origin_instance", "", "Resolve container builds from an origin instance ID.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		provided := 0
		if *githubJob != 0 {
			provided++
		}
		if *buildkiteJob != "" {
			provided++
		}
		if *originInstance != "" {
			provided++
		}
		if provided == 0 {
			return fnerrors.BadInputError("one of --github_job, --buildkite_job, or --origin_instance is required")
		}
		if provided > 1 {
			return fnerrors.BadInputError("workflow flags are mutually exclusive")
		}
		if *output != "plain" && *output != "json" {
			return fnerrors.BadInputError("unsupported output format %q, supported values: plain, json", *output)
		}

		instanceID := *originInstance

		if *githubJob != 0 {
			var err error
			instanceID, err = resolveGithubJobInstance(ctx, *githubJob)
			if err != nil {
				return err
			}
			if instanceID == "" {
				return fnerrors.Newf("job %d has no assigned instance", *githubJob)
			}
		}

		if *buildkiteJob != "" {
			var err error
			instanceID, err = resolveBuildkiteJobInstance(ctx, *buildkiteJob)
			if err != nil {
				return err
			}
			if instanceID == "" {
				return fnerrors.Newf("job %s has no assigned instance", *buildkiteJob)
			}
		}

		refs, err := resolveBuildsFromInstance(ctx, instanceID)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)
		if *output == "json" {
			return json.NewEncoder(stdout).Encode(refs)
		}
		for _, ref := range refs {
			fmt.Fprintln(stdout, ref)
		}
		return nil
	})

	return cmd
}

func resolveGithubJobInstance(ctx context.Context, jobID int64) (string, error) {
	client, err := fnapi.NewJobsServiceClient(ctx)
	if err != nil {
		return "", err
	}

	response, err := client.DescribeJob(ctx, connect.NewRequest(&githubv1beta.DescribeJobRequest{JobId: jobID}))
	if err != nil {
		return "", err
	}
	return response.Msg.GetJob().GetRunner().GetInstanceId(), nil
}

func resolveBuildkiteJobInstance(ctx context.Context, jobID string) (string, error) {
	client, err := fnapi.NewBuildkiteJobsServiceClient(ctx)
	if err != nil {
		return "", err
	}

	response, err := client.DescribeJob(ctx, connect.NewRequest(&buildkitepb.DescribeJobRequest{JobId: jobID}))
	if err != nil {
		return "", err
	}
	return response.Msg.GetJob().GetRunner().GetInstanceId(), nil
}

func resolveBuildsFromInstance(ctx context.Context, instanceID string) ([]string, error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	client, err := compute.NewClient(ctx, token)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	instance, err := client.Compute.DescribeInstance(ctx, &computev1beta.DescribeInstanceRequest{InstanceId: instanceID})
	if err != nil {
		return nil, err
	}
	return buildRefsFromAttachments(instance.GetAttachments()), nil
}

func buildRefsFromAttachments(attachments []*computev1beta.Attachment) []string {
	refs := []string{}
	seen := map[string]struct{}{}
	for _, attachment := range attachments {
		if attachment.GetTypeUrl() == buildRefAttachmentTypeURL {
			ref := string(attachment.GetContent())
			if _, ok := seen[ref]; !ok {
				refs = append(refs, ref)
				seen[ref] = struct{}{}
			}
		}
	}
	return refs
}
