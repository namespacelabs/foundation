// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	computev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/compute/v1beta"
	"buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/stdlib"
	"github.com/araddon/dateparse"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
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

	start := cmd.Flags().String("start", "", "Start time of the report.")
	end := cmd.Flags().String("end", "", "End time of the report, defaults to now if not provided.")
	outPath := cmd.Flags().String("out", "", "Output file path. Creates a temporary file in /tmp if not provided. Use '-' for stdout.")

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
	var profileArgs []string
	var profileExcArgs []string
	var buildkiteOrgIDArgs []string
	var buildkiteOrgIDExcArgs []string
	var buildkitePipelineIDArgs []string
	var buildkitePipelineIDExcArgs []string
	var buildkitePipelineSlugArgs []string
	var buildkitePipelineSlugExcArgs []string
	var buildkiteRepositoryArgs []string
	var buildkiteRepositoryExcArgs []string
	var buildkiteBranchArgs []string
	var buildkiteBranchExcArgs []string
	var buildkiteJobNameArgs []string
	var buildkiteJobNameExcArgs []string
	var buildkiteJobStateArgs []string
	var buildkiteJobStateExcArgs []string

	cmd.Flags().StringSliceVar(&platformArgs, "platform", nil, "platform(s) to include (repeatable). Cannot be passed together with --exclude-platform.")
	cmd.Flags().StringSliceVar(&platformExcArgs, "exclude-platform", nil, "platform(s) to exclude (repeatable). Cannot be passed together with --platform.")

	cmd.Flags().StringSliceVar(&shapeArgs, "shape", nil, "shape(s) to include (repeatable). Cannot be passed together with --exclude-shape.")
	cmd.Flags().StringSliceVar(&shapeExcArgs, "exclude-shape", nil, "shape(s) to exclude (repeatable). Cannot be passed together with --shape.")

	cmd.Flags().StringSliceVar(&purposeArgs, "purpose", nil, "purpose(s) to include (repeatable). Cannot be passed together with --exclude-purpose.")
	cmd.Flags().StringSliceVar(&purposeExcArgs, "exclude-purpose", nil, "purpose(s) to exclude (repeatable). Cannot be passed together with --purpose.")

	cmd.Flags().StringSliceVar(&repoArgs, "repository", nil, "GitHub repositories to include (repeatable). Cannot be passed together with --exclude-repository.")
	cmd.Flags().StringSliceVar(&repoExcArgs, "exclude-repository", nil, "GitHub repositories to exclude (repeatable). Cannot be passed together with --repository.")

	cmd.Flags().StringSliceVar(&branchArgs, "branch", nil, "GitHub branches to include (repeatable). Cannot be passed together with --exclude-branch.")
	cmd.Flags().StringSliceVar(&branchExcArgs, "exclude-branch", nil, "GitHub branches to exclude (repeatable). Cannot be passed together with --branch.")

	cmd.Flags().StringSliceVar(&workflowArgs, "workflow", nil, "GitHub workflows to include (repeatable). Cannot be passed together with --exclude-workflow.")
	cmd.Flags().StringSliceVar(&workflowExcArgs, "exclude-workflow", nil, "GitHub workflows to exclude (repeatable). Cannot be passed together with --workflow.")

	cmd.Flags().StringSliceVar(&jobnameArgs, "jobname", nil, "GitHub job names to include (repeatable). Cannot be passed together with --exclude-jobname.")
	cmd.Flags().StringSliceVar(&jobnameExcArgs, "exclude-jobname", nil, "GitHub job names to exclude (repeatable). Cannot be passed together with --jobname.")

	cmd.Flags().StringSliceVar(&profileArgs, "profile", nil, "Profiles to include (repeatable). Cannot be passed together with --exclude-profile.")
	cmd.Flags().StringSliceVar(&profileExcArgs, "exclude-profile", nil, "Profiles to exclude (repeatable). Cannot be passed together with --profile.")

	cmd.Flags().StringSliceVar(&buildkiteOrgIDArgs, "buildkite-org-id", nil, "Buildkite organization IDs to include (repeatable). Cannot be passed together with --exclude-buildkite-org-id.")
	cmd.Flags().StringSliceVar(&buildkiteOrgIDExcArgs, "exclude-buildkite-org-id", nil, "Buildkite organization IDs to exclude (repeatable). Cannot be passed together with --buildkite-org-id.")

	cmd.Flags().StringSliceVar(&buildkitePipelineIDArgs, "buildkite-pipeline-id", nil, "Buildkite pipeline IDs to include (repeatable). Cannot be passed together with --exclude-buildkite-pipeline-id.")
	cmd.Flags().StringSliceVar(&buildkitePipelineIDExcArgs, "exclude-buildkite-pipeline-id", nil, "Buildkite pipeline IDs to exclude (repeatable). Cannot be passed together with --buildkite-pipeline-id.")

	cmd.Flags().StringSliceVar(&buildkitePipelineSlugArgs, "buildkite-pipeline-slug", nil, "Buildkite pipeline slugs to include (repeatable). Cannot be passed together with --exclude-buildkite-pipeline-slug.")
	cmd.Flags().StringSliceVar(&buildkitePipelineSlugExcArgs, "exclude-buildkite-pipeline-slug", nil, "Buildkite pipeline slugs to exclude (repeatable). Cannot be passed together with --buildkite-pipeline-slug.")

	cmd.Flags().StringSliceVar(&buildkiteRepositoryArgs, "buildkite-repository", nil, "Buildkite repositories to include (repeatable). Cannot be passed together with --exclude-buildkite-repository.")
	cmd.Flags().StringSliceVar(&buildkiteRepositoryExcArgs, "exclude-buildkite-repository", nil, "Buildkite repositories to exclude (repeatable). Cannot be passed together with --buildkite-repository.")

	cmd.Flags().StringSliceVar(&buildkiteBranchArgs, "buildkite-branch", nil, "Buildkite branches to include (repeatable). Cannot be passed together with --exclude-buildkite-branch.")
	cmd.Flags().StringSliceVar(&buildkiteBranchExcArgs, "exclude-buildkite-branch", nil, "Buildkite branches to exclude (repeatable). Cannot be passed together with --buildkite-branch.")

	cmd.Flags().StringSliceVar(&buildkiteJobNameArgs, "buildkite-job-name", nil, "Buildkite job names to include (repeatable). Cannot be passed together with --exclude-buildkite-job-name.")
	cmd.Flags().StringSliceVar(&buildkiteJobNameExcArgs, "exclude-buildkite-job-name", nil, "Buildkite job names to exclude (repeatable). Cannot be passed together with --buildkite-job-name.")

	cmd.Flags().StringSliceVar(&buildkiteJobStateArgs, "buildkite-job-state", nil, "Buildkite job states to include (repeatable). Cannot be passed together with --exclude-buildkite-job-state.")
	cmd.Flags().StringSliceVar(&buildkiteJobStateExcArgs, "exclude-buildkite-job-state", nil, "Buildkite job states to exclude (repeatable). Cannot be passed together with --buildkite-job-state.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *start == "" {
			return fnerrors.New("--start is required. Example:\n    nsc instance report --start \"2026-01-01\" --end \"2026-01-02\"")
		}

		startTs, err := dateparse.ParseAny(*start)
		if err != nil {
			return fnerrors.Newf("invalid --start timestamp: %w", err)
		}

		var endTs time.Time
		if *end == "" {
			endTs = time.Now()
		} else {
			endTs, err = dateparse.ParseAny(*end)
			if err != nil {
				return fnerrors.Newf("invalid --end timestamp: %w", err)
			}
		}

		var outFile *os.File
		var outName string
		switch *outPath {
		case "-":
			outFile = os.Stdout
			outName = "stdout"
		case "":
			tmp, err := os.CreateTemp("", "nsc-report-*.csv")
			if err != nil {
				return fnerrors.Newf("Failed to create temp file: %w", err)
			}
			outFile = tmp
			outName = tmp.Name()
		default:
			file, err := os.Create(*outPath)
			if err != nil {
				return fnerrors.Newf("Failed to create output file %s: %w", *outPath, err)
			}
			outFile = file
			outName = *outPath
		}

		fmt.Fprintf(console.Stdout(ctx), "Writing output to path: %s\n", outName)

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

		profileMatcher, err := createMatcher("profile", profileArgs, profileExcArgs)
		if err != nil {
			return err
		}
		filter.GithubProfile = profileMatcher

		buildkiteOrgIDMatcher, err := createMatcher("buildkite-org-id", buildkiteOrgIDArgs, buildkiteOrgIDExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkiteOrgId = buildkiteOrgIDMatcher

		buildkitePipelineIDMatcher, err := createMatcher("buildkite-pipeline-id", buildkitePipelineIDArgs, buildkitePipelineIDExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkitePipelineId = buildkitePipelineIDMatcher

		buildkitePipelineSlugMatcher, err := createMatcher("buildkite-pipeline-slug", buildkitePipelineSlugArgs, buildkitePipelineSlugExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkitePipelineSlug = buildkitePipelineSlugMatcher

		buildkiteRepositoryMatcher, err := createMatcher("buildkite-repository", buildkiteRepositoryArgs, buildkiteRepositoryExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkiteRepository = buildkiteRepositoryMatcher

		buildkiteBranchMatcher, err := createMatcher("buildkite-branch", buildkiteBranchArgs, buildkiteBranchExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkiteBranch = buildkiteBranchMatcher

		buildkiteJobNameMatcher, err := createMatcher("buildkite-job-name", buildkiteJobNameArgs, buildkiteJobNameExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkiteJobName = buildkiteJobNameMatcher

		buildkiteJobStateMatcher, err := createMatcher("buildkite-job-state", buildkiteJobStateArgs, buildkiteJobStateExcArgs)
		if err != nil {
			return err
		}
		filter.BuildkiteJobState = buildkiteJobStateMatcher

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

		rowsFile, err := os.CreateTemp("", "nsc-report-rows-*.csv")
		if err != nil {
			return fnerrors.Newf("Failed to create temporary report file: %w", err)
		}
		defer os.Remove(rowsFile.Name())
		defer rowsFile.Close()

		rowsWriter := csv.NewWriter(rowsFile)
		includeGithub := false
		includeBuildkite := false

		for {
			msg, err := resp.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fnerrors.Newf("Error %w", err)
			}
			for _, entry := range msg.Entries {
				includeGithub = includeGithub || entry.GetGithubJob() != nil
				includeBuildkite = includeBuildkite || entry.GetBuildkiteJob() != nil
				if err := rowsWriter.Write(entryToRecords(entry)); err != nil {
					return fnerrors.Newf("Unable to write temporary report: %w", err)
				}
			}
		}
		rowsWriter.Flush()
		if err := rowsWriter.Error(); err != nil {
			return fnerrors.Newf("Unable to write temporary report: %w", err)
		}
		if _, err := rowsFile.Seek(0, io.SeekStart); err != nil {
			return fnerrors.Newf("Unable to read temporary report: %w", err)
		}

		csvWriter := csv.NewWriter(outFile)
		if err := csvWriter.Write(reportHeader(includeGithub, includeBuildkite)); err != nil {
			return fnerrors.Newf("Unable to write report: %w", err)
		}

		rowsReader := csv.NewReader(rowsFile)
		for {
			record, err := rowsReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fnerrors.Newf("Unable to read temporary report: %w", err)
			}
			if err := csvWriter.Write(selectReportColumns(record, includeGithub, includeBuildkite)); err != nil {
				return fnerrors.Newf("Unable to write report: %w", err)
			}
		}
		csvWriter.Flush()
		if err := csvWriter.Error(); err != nil {
			return fnerrors.Newf("Unable to write report: %w", err)
		}
		return nil
	})

	return cmd

}

var baseReportHeader = []string{
	"instance_id",
	"created_at",
	"started_at",
	"destroyed_at",
	"resources_cpu",
	"resources_ram_gb",
	"resources_cpu_actual_max",
	"resources_ram_gb_actual_max_percent",
	"cache_volume_hit",
}

var githubReportHeader = []string{
	"github_job_id",
	"github_job_name",
	"github_job_workflow_name",
	"github_run_id",
	"github_run_attempt",
	"job_created_at",
	"job_started_at",
	"job_completed_at",
	"profile",
	"repository",
	"branch",
	"sender_login",
	"conclusion",
}

var buildkiteReportHeader = []string{
	"buildkite_job_id",
	"buildkite_job_name",
	"buildkite_pipeline_slug",
	"buildkite_pipeline_id",
	"buildkite_build_id",
	"buildkite_build_number",
	"buildkite_job_created_at",
	"buildkite_job_runnable_at",
	"buildkite_job_started_at",
	"buildkite_job_finished_at",
	"buildkite_org_id",
	"buildkite_repository",
	"buildkite_branch",
	"buildkite_job_state",
}

func reportHeader(includeGithub, includeBuildkite bool) []string {
	header := append([]string{}, baseReportHeader...)
	if includeGithub {
		header = append(header, githubReportHeader...)
	}
	if includeBuildkite {
		header = append(header, buildkiteReportHeader...)
	}
	return header
}

func selectReportColumns(record []string, includeGithub, includeBuildkite bool) []string {
	baseEnd := len(baseReportHeader)
	githubEnd := baseEnd + len(githubReportHeader)
	selected := append([]string{}, record[:baseEnd]...)
	if includeGithub {
		selected = append(selected, record[baseEnd:githubEnd]...)
	}
	if includeBuildkite {
		selected = append(selected, record[githubEnd:]...)
	}
	return selected
}

func entryToRecords(entry *computev1beta.InstanceReportEntry) []string {
	cacheHit := false
	for _, v := range entry.GetVolumes() {
		cacheHit = cacheHit || v.CacheHit
	}
	githubJob := entry.GetGithubJob()
	buildkiteJob := entry.GetBuildkiteJob()
	cols := []string{entry.InstanceId,
		tsToString(entry.GetCreatedAt()),
		tsToString(entry.GetStartedAt()),
		tsToString(entry.GetDestroyedAt()),
		strconv.FormatFloat(float64(entry.GetResourcesCpu()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesRamGb()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesCpuActualMax()), 'f', -1, 32),
		strconv.FormatFloat(float64(entry.GetResourcesRamGbActualMaxPercent()), 'f', -1, 32),
		strconv.FormatBool(cacheHit),
		strconv.FormatInt(githubJob.GetJobId(), 10),
		githubJob.GetJobName(),
		githubJob.GetWorkflowName(),
		strconv.FormatInt(githubJob.GetRunId(), 10),
		strconv.FormatInt(githubJob.GetRunAttempt(), 10),
		tsToString(githubJob.GetJobCreatedAt()),
		tsToString(githubJob.GetJobStartedAt()),
		tsToString(githubJob.GetJobCompletedAt()),
		githubJob.GetProfile(),
		githubJob.GetRepository(),
		githubJob.GetBranch(),
		githubJob.GetSenderLogin(),
		githubJob.GetConclusion(),
		buildkiteJob.GetJobId(),
		buildkiteJob.GetJobName(),
		buildkiteJob.GetPipelineSlug(),
		buildkiteJob.GetPipelineId(),
		buildkiteJob.GetBuildId(),
		strconv.FormatInt(buildkiteJob.GetBuildNumber(), 10),
		tsToString(buildkiteJob.GetJobCreatedAt()),
		tsToString(buildkiteJob.GetJobRunnableAt()),
		tsToString(buildkiteJob.GetJobStartedAt()),
		tsToString(buildkiteJob.GetJobFinishedAt()),
		buildkiteJob.GetOrgId(),
		buildkiteJob.GetRepository(),
		buildkiteJob.GetBranch(),
		buildkiteJob.GetJobState(),
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
