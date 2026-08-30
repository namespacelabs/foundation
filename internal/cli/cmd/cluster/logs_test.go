// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestVisibleLogLabel(t *testing.T) {
	testCases := []struct {
		name              string
		labels            map[string]string
		stream            string
		includeInstanceID bool
		want              string
	}{
		{
			name:              "uses pod label for kubernetes logs",
			labels:            map[string]string{k8sPodNameLogLabel: "api-123", namespaceLogLabel: "default", systemLogLabel: "kernel"},
			stream:            "stdout",
			includeInstanceID: false,
			want:              "api-123",
		},
		{
			name:              "includes non-default namespace in pod label",
			labels:            map[string]string{k8sPodNameLogLabel: "api-123", namespaceLogLabel: "tenant-a"},
			stream:            "stdout",
			includeInstanceID: false,
			want:              "tenant-a/api-123",
		},
		{
			name:              "falls back to system label when stream is empty",
			labels:            map[string]string{systemLogLabel: "kernel"},
			stream:            "",
			includeInstanceID: false,
			want:              "kernel",
		},
		{
			name:              "falls back to stream when there is no richer label",
			labels:            map[string]string{},
			stream:            "stderr",
			includeInstanceID: false,
			want:              "stderr",
		},
		{
			name:              "prefixes instance id for pod labels in all-instance mode",
			labels:            map[string]string{instanceIDLogLabel: "6adu2jbpsmbb0", k8sPodNameLogLabel: "api-123"},
			stream:            "stdout",
			includeInstanceID: true,
			want:              "6adu2jbpsmbb0/api-123",
		},
		{
			name:              "prefixes instance id for system labels in all-instance mode",
			labels:            map[string]string{instanceIDLogLabel: "6adu2jbpsmbb0", systemLogLabel: "kernel"},
			stream:            "",
			includeInstanceID: true,
			want:              "6adu2jbpsmbb0/kernel",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := visibleLogLabel(tc.labels, tc.stream, tc.includeInstanceID)
			if got != tc.want {
				t.Fatalf("visibleLogLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLogsCmdRequiresTimeframeForAllInstances(t *testing.T) {
	cmd := NewLogsCmd()
	cmd.SetArgs([]string{"--all"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("ExecuteContext() error = nil, want missing timeframe error")
	}

	if !strings.Contains(err.Error(), "--since, --after, or --before is required with --all") {
		t.Fatalf("ExecuteContext() error = %q, want missing timeframe error", err)
	}
}
