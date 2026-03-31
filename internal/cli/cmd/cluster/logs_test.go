// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import "testing"

func TestVisibleLogLabel(t *testing.T) {
	testCases := []struct {
		name   string
		labels map[string]string
		stream string
		want   string
	}{
		{
			name:   "uses pod label for kubernetes logs",
			labels: map[string]string{k8sPodNameLogLabel: "api-123", namespaceLogLabel: "default", systemLogLabel: "kernel"},
			stream: "stdout",
			want:   "api-123",
		},
		{
			name:   "includes non-default namespace in pod label",
			labels: map[string]string{k8sPodNameLogLabel: "api-123", namespaceLogLabel: "tenant-a"},
			stream: "stdout",
			want:   "tenant-a/api-123",
		},
		{
			name:   "falls back to system label when stream is empty",
			labels: map[string]string{systemLogLabel: "kernel"},
			stream: "",
			want:   "kernel",
		},
		{
			name:   "falls back to stream when there is no richer label",
			labels: map[string]string{},
			stream: "stderr",
			want:   "stderr",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := visibleLogLabel(tc.labels, tc.stream)
			if got != tc.want {
				t.Fatalf("visibleLogLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}
