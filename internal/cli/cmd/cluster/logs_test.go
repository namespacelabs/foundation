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
