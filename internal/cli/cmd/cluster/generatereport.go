// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"strconv"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api/compute"
	"namespacelabs.dev/integrations/auth"
)

func NewGenerateReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generates a report of compute instances.",
		Args:  cobra.NoArgs, // for now
	}

	start := cmd.Flags().String("start", "", "Start time of the report (RFC3339, e.g. 2024-01-15T10:30:00Z).")
	end := cmd.Flags().String("end", "", "End time of the report, defaults to now (RFC3339, e.g. 2024-01-15T10:30:00Z).")

	// Args for filtering. We allow multiple strings per filter type. Each matcher can have
	// either <matcher>Args or <matcher>ExcArgs nonempty, depending on if the filter should
	// include or exclude the respective arguments.
	var platformArgs []string
	var platformExcArgs []string
	var shapeArgs []string
	var shapeExcArgs []string
	var purposeArgs []string
	var purposeExcArgs []string
	var repoArgs []string
	var repoExcArgs []string
	var branchArgs []string
	var branchExcArgs []string
	var workflowArgs []string
	var workflowExcArgs []string
	var jobnameArgs []string
	var jobnameExcArgs []string

	cmd.Flags().StringSliceVar(&platformArgs, "platform", nil, "platform(s) to include (repeatable). Cannot be passed together with --exclude-platform.")
	cmd.Flags().StringSliceVar(&platformExcArgs, "exclude-platform", nil, "platform(s) to exclude (repeatable). Cannot be passed together with --platform.")

	cmd.Flags().StringSliceVar(&shapeArgs, "shape", nil, "shape(s) to include (repeatable). Cannot be passed together with --exclude-shape.")
	cmd.Flags().StringSliceVar(&shapeExcArgs, "exclude-shape", nil, "shape(s) to exclude (repeatable). Cannot be passed together with --shape.")

	cmd.Flags().StringSliceVar(&purposeArgs, "purpose", nil, "purpose(s) to include (repeatable). Cannot be passed together with --exclude-purpose.")
	cmd.Flags().StringSliceVar(&purposeExcArgs, "exclude-purpose", nil, "purpose(s) to exclude (repeatable). Cannot be passed together with --purpose.")

	cmd.Flags().StringSliceVar(&repoArgs, "repository", nil, "Github repository/ies to include (repeatable). Cannot be passed together with --exclude-repository.")
	cmd.Flags().StringSliceVar(&repoExcArgs, "exclude-repository", nil, "Github repository/ies to exclude (repeatable). Cannot be passed together with --repository.")

	cmd.Flags().StringSliceVar(&branchArgs, "branch", nil, "Github branch(es) to include (repeatable). Cannot be passed together with --exclude-branch.")
	cmd.Flags().StringSliceVar(&branchExcArgs, "exclude-branch", nil, "Github branch(es) to exclude (repeatable). Cannot be passed together with --branch.")

	cmd.Flags().StringSliceVar(&workflowArgs, "workflow", nil, "Github workflow(s) to include (repeatable). Cannot be passed together with --exclude-workflow.")
	cmd.Flags().StringSliceVar(&workflowExcArgs, "exclude-workflow", nil, "Github workflow(s) to exclude (repeatable). Cannot be passed together with --workflow.")

	cmd.Flags().StringSliceVar(&jobnameArgs, "jobname", nil, "Github jobname(s) to include (repeatable). Cannot be passed together with --exclude-jobname.")
	cmd.Flags().StringSliceVar(&jobnameExcArgs, "exclude-jobname", nil, "Github jobname(s) to exclude (repeatable). Cannot be passed together with --jobname.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *start == "" {
			return fnerrors.Newf("--start is required")
		}

		startTs, err := time.Parse(time.RFC3339, *start)
		if err != nil {
			return fnerrors.Newf("invalid --start timestamp: %w", err)
		}

		var endTs time.Time
		if *end == "" {
			endTs = time.Now()
		} else {
			endTs, err = time.Parse(time.RFC3339, *end)
			if err != nil {
				return fnerrors.Newf("invalid --end timestamp: %w", err)
			}
		}

		filter := &computev1beta.GetUsageTimeSeriesRequest_UsageFilter{}

		platformMatcher, err := createMatcher("platform", platformArgs, platformExcArgs)
		if err != nil {
			return err
		}
		filter.Platform = platformMatcher

		shapeMatcher, err := createMatcher("shape", shapeArgs, shapeExcArgs)
		if err != nil {
			return err
		}
		filter.Shape = shapeMatcher

		purposeMatcher, err := createMatcher("purpose", purposeArgs, purposeExcArgs)
		if err != nil {
			return err
		}
		filter.Purpose = purposeMatcher

		repoMatcher, err := createMatcher("repository", repoArgs, repoExcArgs)
		if err != nil {
			return err
		}
		filter.GithubRepository = repoMatcher

		branchMatcher, err := createMatcher("branch", branchArgs, branchExcArgs)
		if err != nil {
			return err
		}
		filter.GithubBranch = branchMatcher

		workflowMatcher, err := createMatcher("workflow", workflowArgs, workflowExcArgs)
		if err != nil {
			return err
		}
		filter.GithubWorkflowName = workflowMatcher

		jobnameMatcher, err := createMatcher("jobname", jobnameArgs, jobnameExcArgs)
		if err != nil {
			return err
		}
		filter.GithubJobName = jobnameMatcher

		token, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.Newf("Authentication error %w", err)
		}

		client, err := compute.NewClient(ctx, token)
		if err != nil {
			return fnerrors.Newf("Connection error %w", err)
		}
		defer client.Close()

		req := &computev1beta.GenerateReportRequest{
			StartTime: timestamppb.New(startTs),
			EndTime:   timestamppb.New(endTs),
			Filter:    filter,
		}

		resp, err := client.Usage.GenerateReport(ctx, req)
		if err != nil {
			return fnerrors.Newf("Unable to generate report: %w", err)
		}

		csvWriter := csv.NewWriter(os.Stdout)

		header := []string{
			"instance_id",
			"created_at",
			"started_at",
			"destroyed_at",
			"resources_cpu",
			"resources_ram_gb",
			"resources_cpu_actual_max",
			"resources_ram_gb_actual_max_percent",
			"github_job_id",
			"github_job_name",
			"github_job_workflow_name",
			"github_run_id",
			"github_run_attempt",
			"job_created_at",
			"job_started_at",
			"job_completed_at",
		}

		csvWriter.Write(header)

		for {
			msg, err := resp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fnerrors.Newf("Error %w", err)
			}
			for _, entry := range msg.Entries {
				csvWriter.Write(entryToRecords(entry))
			}
		}
		csvWriter.Flush()
		return nil
	})

	return cmd

}

func entryToRecords(entry *computev1beta.InstanceReportEntry) []string {
	githubJob := entry.GetGithubJob()
	cols := []string{entry.InstanceId,
		tsToString(entry.GetCreatedAt()),
		tsToString(entry.GetStartedAt()),
		tsToString(entry.GetDestroyedAt()),
		strconv.FormatFloat(float64(entry.GetResourcesCpu()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesRamGb()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesCpuActualMax()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesRamGbActualMaxPercent()), 'f', -1, 32),
		strconv.FormatInt(githubJob.GetJobId(), 10),
		githubJob.GetJobName(),
		githubJob.GetWorkflowName(),
		strconv.FormatInt(githubJob.GetRunId(), 10),
		strconv.FormatInt(githubJob.GetRunAttempt(), 10),
		tsToString(githubJob.GetJobCreatedAt()),
		tsToString(githubJob.GetJobStartedAt()),
		tsToString(githubJob.GetJobCompletedAt()),
	}
	return cols
}

// Safely convert timestamp to UTC string
func tsToString(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().String()
}

func createMatcher(field string, args []string, excArgs []string) (*stdlib.StringMatcher, error) {
	if args != nil && excArgs != nil {
		err := fnerrors.Newf("At most one of --%s or --exclude-%s may be passed.", field, field)
		return nil, err
	} else if args != nil {
		return &stdlib.StringMatcher{
			Values: args,
			Op:     stdlib.StringMatcher_IS_ANY_OF,
		}, nil
	} else if excArgs != nil {
		return &stdlib.StringMatcher{
			Values: excArgs,
			Op:     stdlib.StringMatcher_IS_NOT,
		}, nil
	}
	return nil, nil
}
