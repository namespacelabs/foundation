// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"expired", -1 * time.Hour, "expired"},
		{"zero", 0, "0m"},
		{"minutes only", 30 * time.Minute, "30m"},
		{"one hour", 1 * time.Hour, "1h"},
		{"hours and minutes", 2*time.Hour + 30*time.Minute, "2h30m"},
		{"hours no minutes", 5 * time.Hour, "5h"},
		{"one day", 24 * time.Hour, "1d"},
		{"days and hours", 25 * time.Hour, "1d1h"},
		{"days no hours", 48 * time.Hour, "2d"},
		{"multiple days and hours", 50 * time.Hour, "2d2h"},
		{"almost a day", 23*time.Hour + 59*time.Minute, "23h59m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, got, tt.expected)
			}
		})
	}
}
