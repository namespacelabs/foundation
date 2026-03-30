// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import "testing"

func TestVisibleLogLabel(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		stream string
		source string
		want   string
	}{
		{
			name: "pod label keeps non-default namespace",
			labels: map[string]string{
				namespaceLogLabel:  "buildkite",
				k8sPodNameLogLabel: "agent-0",
			},
			want: "buildkite/agent-0",
		},
		{
			name:   "stream fallback",
			stream: "stderr",
			want:   "stderr",
		},
		{
			name: "system fallback",
			labels: map[string]string{
				systemLogLabel: "kernel",
			},
			source: "kmsg",
			want:   "kernel",
		},
		{
			name:   "source fallback",
			source: "containers",
			want:   "containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visibleLogLabel(tt.labels, tt.stream, tt.source); got != tt.want {
				t.Fatalf("visibleLogLabel(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogicalLogLines(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "empty",
			content: "",
			want:    []string{""},
		},
		{
			name:    "carriage returns become new lines",
			content: "Receiving 10%\rReceiving 20%\rDone\r",
			want:    []string{"Receiving 10%", "Receiving 20%", "Done"},
		},
		{
			name:    "mixed newline styles",
			content: "a\r\nb\nc",
			want:    []string{"a", "b", "c"},
		},
		{
			name:    "preserve empty interior lines",
			content: "a\n\nb",
			want:    []string{"a", "", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logicalLogLines(tt.content)
			if len(got) != len(tt.want) {
				t.Fatalf("logicalLogLines(%q) len=%d, want %d (%q)", tt.content, len(got), len(tt.want), got)
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("logicalLogLines(%q)[%d] = %q, want %q", tt.content, i, got[i], tt.want[i])
				}
			}
		})
	}
}
