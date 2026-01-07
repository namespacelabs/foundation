// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/github/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/tui"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewJobsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage GitHub Actions jobs.",
	}

	cmd.AddCommand(newJobsListCmd())

	return cmd
}

func newJobsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List GitHub Actions jobs.",
		Args:  cobra.NoArgs,
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	maxEntries := cmd.Flags().Int64("max_entries", 50, "Maximum number of jobs to return.")
	repository := cmd.Flags().String("repository", "", "Filter by repository (e.g., 'owner/repo').")
	conclusion := cmd.Flags().String("conclusion", "", "Filter by conclusion (e.g., 'success', 'failure', 'cancelled').")
	senderLogin := cmd.Flags().String("sender_login", "", "Filter by the GitHub login of the user who triggered the workflow.")
	since := fncobra.Duration(cmd.Flags(), "since", 0, "Filter jobs created after this duration ago (e.g., '24h', '7d').")
	pending := cmd.Flags().Bool("pending", false, "Only show jobs that have not started yet.")
	running := cmd.Flags().Bool("running", false, "Only show jobs that are currently running (started but not finished).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		req := &v1beta.ListJobsRequest{
			MaxEntries: *maxEntries,
		}

		if *repository != "" {
			req.Repository = &stdlib.StringMatcher{
				Values: []string{*repository},
			}
		}

		if *conclusion != "" {
			req.Conclusion = &stdlib.StringMatcher{
				Values: []string{*conclusion},
			}
		}

		if *senderLogin != "" {
			req.SenderLogin = &stdlib.StringMatcher{
				Values: []string{*senderLogin},
			}
		}

		if *since > 0 {
			afterTime := time.Now().Add(-*since)
			req.TimeRange = &stdlib.TimestampRange{
				After: timestamppb.New(afterTime),
			}
		}

		if *pending {
			req.HasNotStarted = true
		}

		if *running {
			req.HasNotFinished = true
		}

		jobs, err := listJobs(ctx, req)
		if err != nil {
			return err
		}

		stdout := console.Stdout(ctx)

		if *output == "json" {
			enc := json.NewEncoder(stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(transformJobs(jobs)); err != nil {
				return fnerrors.InternalError("failed to encode jobs as JSON output: %w", err)
			}
			return nil
		}

		if len(jobs) == 0 {
			fmt.Fprintf(stdout, "No jobs found.\n")
			return nil
		}

		cols := []tui.Column{
			{Key: "job_id", Title: "Job ID", MinWidth: 10, MaxWidth: 15},
			{Key: "repository", Title: "Repository", MinWidth: 15, MaxWidth: 30},
			{Key: "workflow", Title: "Workflow", MinWidth: 10, MaxWidth: 25},
			{Key: "job_name", Title: "Job", MinWidth: 10, MaxWidth: 25},
			{Key: "conclusion", Title: "Status", MinWidth: 8, MaxWidth: 12},
			{Key: "sender", Title: "Sender", MinWidth: 8, MaxWidth: 20},
			{Key: "created_at", Title: "Created", MinWidth: 15, MaxWidth: 25},
		}

		rows := []tui.Row{}
		for _, job := range jobs {
			status := job.Conclusion
			if status == "" {
				if job.StartedAt != nil {
					status = "running"
				} else {
					status = "pending"
				}
			}

			createdAt := "-"
			if job.CreatedAt != nil {
				createdAt = job.CreatedAt.AsTime().Format(time.RFC3339)
			}

			rows = append(rows, tui.Row{
				"job_id":     fmt.Sprintf("%d", job.JobId),
				"repository": job.Repository,
				"workflow":   job.WorkflowName,
				"job_name":   job.JobName,
				"conclusion": status,
				"sender":     job.SenderLogin,
				"created_at": createdAt,
			})
		}

		return tui.StaticTable(ctx, cols, rows)
	})

	return cmd
}

func listJobs(ctx context.Context, req *v1beta.ListJobsRequest) ([]*v1beta.Job, error) {
	client, err := fnapi.NewJobsServiceClient(ctx)
	if err != nil {
		return nil, err
	}

	res, err := client.ListJobs(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, err
	}

	return res.Msg.Jobs, nil
}

func transformJobs(jobs []*v1beta.Job) []map[string]any {
	var result []map[string]any
	for _, job := range jobs {
		result = append(result, transformJobForOutput(job))
	}
	return result
}

func transformJobForOutput(job *v1beta.Job) map[string]any {
	m := map[string]any{
		"job_id":        job.JobId,
		"repository":    job.Repository,
		"run_id":        job.RunId,
		"workflow_name": job.WorkflowName,
		"job_name":      job.JobName,
		"sender_login":  job.SenderLogin,
		"ref":           job.Ref,
	}

	if job.Conclusion != "" {
		m["conclusion"] = job.Conclusion
	}

	if job.JobUrl != "" {
		m["job_url"] = job.JobUrl
	}

	if job.CreatedAt != nil {
		m["created_at"] = job.CreatedAt.AsTime()
	}

	if job.QueuedAt != nil {
		m["queued_at"] = job.QueuedAt.AsTime()
	}

	if job.StartedAt != nil {
		m["started_at"] = job.StartedAt.AsTime()
	}

	if job.CompletedAt != nil {
		m["completed_at"] = job.CompletedAt.AsTime()
	}

	if job.Runner != nil {
		m["runner"] = map[string]any{
			"runner_id":      job.Runner.RunnerId,
			"runner_name":    job.Runner.RunnerName,
			"instance_id":    job.Runner.InstanceId,
			"container_name": job.Runner.ContainerName,
		}
	}

	if job.Profile != nil {
		m["profile"] = map[string]any{
			"tag": job.Profile.Tag,
		}
	}

	return m
}
