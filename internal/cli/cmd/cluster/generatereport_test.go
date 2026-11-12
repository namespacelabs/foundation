// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	computev1beta "namespacelabs.dev/integrations/proto/namespace/cloud/compute/v1beta"
)

func TestReportColumns(t *testing.T) {
	fullRecord := make([]string, len(baseReportHeader)+len(githubReportHeader)+len(buildkiteReportHeader))
	for i := range fullRecord {
		fullRecord[i] = strconv.Itoa(i)
	}
	baseEnd := len(baseReportHeader)
	githubEnd := baseEnd + len(githubReportHeader)

	for _, test := range []struct {
		name             string
		includeGithub    bool
		includeBuildkite bool
		wantHeader       []string
		wantRecord       []string
	}{
		{name: "base only", wantHeader: baseReportHeader, wantRecord: fullRecord[:baseEnd]},
		{name: "GitHub", includeGithub: true, wantHeader: append(append([]string{}, baseReportHeader...), githubReportHeader...), wantRecord: fullRecord[:githubEnd]},
		{name: "Buildkite", includeBuildkite: true, wantHeader: append(append([]string{}, baseReportHeader...), buildkiteReportHeader...), wantRecord: append(append([]string{}, fullRecord[:baseEnd]...), fullRecord[githubEnd:]...)},
		{name: "both", includeGithub: true, includeBuildkite: true, wantHeader: append(append(append([]string{}, baseReportHeader...), githubReportHeader...), buildkiteReportHeader...), wantRecord: fullRecord},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reportHeader(test.includeGithub, test.includeBuildkite); !reflect.DeepEqual(got, test.wantHeader) {
				t.Errorf("reportHeader() = %v, want %v", got, test.wantHeader)
			}
			if got := selectReportColumns(fullRecord, test.includeGithub, test.includeBuildkite); !reflect.DeepEqual(got, test.wantRecord) {
				t.Errorf("selectReportColumns() = %v, want %v", got, test.wantRecord)
			}
		})
	}
}

func TestWriteInstanceReport(t *testing.T) {
	entries := []*computev1beta.InstanceReportEntry{
		{
			InstanceId: "runner-1",
			CreatedAt:  timestamppb.New(time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)),
			GithubJob: &computev1beta.InstanceReportEntry_GitHubJob{
				JobId: 9007199254740993, JobName: "tests\nunit", Repository: "acme/app", RunAttempt: 2,
			},
			Volumes: []*computev1beta.InstanceReportEntry_Volume{
				{Tag: "checkout", MountPoint: "/checkout", SizeMb: 4096, UsedMb: 123, CacheHit: true},
				{Tag: "deps", MountPoint: "/deps", SizeMb: 8192, UsedMb: 456},
			},
		},
		{
			InstanceId: "runner-2",
			BuildkiteJob: &computev1beta.InstanceReportEntry_BuildkiteJob{
				JobId: "bk-job", PipelineSlug: "build", JobState: "passed",
			},
		},
	}
	for _, format := range []string{"csv", "jsonl"} {
		t.Run(format, func(t *testing.T) {
			next := 0
			var out bytes.Buffer
			err := writeInstanceReport(&out, format, func() (*computev1beta.GenerateReportResponse, error) {
				if next == len(entries) {
					return nil, io.EOF
				}
				entry := entries[next]
				next++
				return &computev1beta.GenerateReportResponse{Entries: []*computev1beta.InstanceReportEntry{entry}}, nil
			})
			assert.NoError(t, err)
			if format == "jsonl" {
				lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
				if !assert.Len(t, lines, 2) {
					return
				}
				var first, second map[string]any
				assert.NoError(t, json.Unmarshal([]byte(lines[0]), &first))
				assert.NoError(t, json.Unmarshal([]byte(lines[1]), &second))
				assert.Equal(t, "runner-1", first["instance_id"])
				assert.Equal(t, "2026-11-01T10:00:00Z", first["created_at"])
				assert.Equal(t, map[string]any{
					"job_id": "9007199254740993", "job_name": "tests\nunit", "repository": "acme/app", "run_attempt": "2",
				}, first["github_job"])
				assert.Equal(t, []any{
					map[string]any{"tag": "checkout", "mount_point": "/checkout", "size_mb": "4096", "used_mb": "123", "cache_hit": true},
					map[string]any{"tag": "deps", "mount_point": "/deps", "size_mb": "8192", "used_mb": "456"},
				}, first["volumes"])
				assert.Equal(t, "runner-2", second["instance_id"])
				assert.Equal(t, map[string]any{"job_id": "bk-job", "pipeline_slug": "build", "job_state": "passed"}, second["buildkite_job"])
			} else {
				rows, err := csv.NewReader(&out).ReadAll()
				assert.NoError(t, err)
				if !assert.Len(t, rows, 3) {
					return
				}
				values := map[string]string{}
				for i, name := range rows[0] {
					values[name] = rows[1][i]
				}
				assert.Equal(t, "true", values["cache_volume_hit"])
				assert.Equal(t, "tests\nunit", values["github_job_name"])
				assert.Equal(t, "9007199254740993", values["github_job_id"])
				assert.Contains(t, rows[0], "buildkite_pipeline_slug")
				assert.Contains(t, rows[2], "bk-job")
			}
		})
	}
}

func TestWriteInstanceReportJSONLEmptyAndErrors(t *testing.T) {
	var out bytes.Buffer
	err := writeInstanceReport(&out, "jsonl", func() (*computev1beta.GenerateReportResponse, error) { return nil, io.EOF })
	assert.NoError(t, err)
	assert.Empty(t, out.String())

	next := 0
	err = writeInstanceReport(&out, "jsonl", func() (*computev1beta.GenerateReportResponse, error) {
		next++
		if next == 1 {
			return &computev1beta.GenerateReportResponse{Entries: []*computev1beta.InstanceReportEntry{{InstanceId: "runner-1"}}}, nil
		}
		return nil, errors.New("stream interrupted")
	})
	assert.ErrorContains(t, err, "output may be partial")
	assert.ErrorContains(t, err, "stream interrupted")
	assert.JSONEq(t, `{"instance_id":"runner-1"}`, strings.TrimSpace(out.String()))

	file, err := os.Open(os.DevNull)
	if !assert.NoError(t, err) {
		return
	}
	defer file.Close()
	err = writeInstanceReport(file, "jsonl", func() (*computev1beta.GenerateReportResponse, error) {
		return &computev1beta.GenerateReportResponse{Entries: []*computev1beta.InstanceReportEntry{{InstanceId: "runner-1"}}}, nil
	})
	assert.ErrorContains(t, err, "Unable to write report")
}

func TestGenerateReportRejectsUnknownFormat(t *testing.T) {
	cmd := NewGenerateReportCmd()
	cmd.SetArgs([]string{"--format", "yaml"})
	assert.ErrorContains(t, cmd.Execute(), "expected csv or jsonl")
}
