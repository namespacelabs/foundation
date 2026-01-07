// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package duration

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		// Standard units (same as time.ParseDuration)
		{"0", 0, false},
		{"1ns", time.Nanosecond, false},
		{"1us", time.Microsecond, false},
		{"1µs", time.Microsecond, false},
		{"1ms", time.Millisecond, false},
		{"1s", time.Second, false},
		{"1m", time.Minute, false},
		{"1h", time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"1.5h", 90 * time.Minute, false},
		{"-1h", -time.Hour, false},
		{"+1h", time.Hour, false},

		// Days
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"7d", 168 * time.Hour, false},
		{"1.5d", 36 * time.Hour, false},
		{"-1d", -24 * time.Hour, false},

		// Weeks
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"0.5w", 84 * time.Hour, false},
		{"-1w", -7 * 24 * time.Hour, false},

		// Combined
		{"1w2d", 9 * 24 * time.Hour, false},
		{"1w1d12h", 204 * time.Hour, false},
		{"2d12h30m", 60*time.Hour + 30*time.Minute, false},
		{"1w2d3h4m5s", 9*24*time.Hour + 3*time.Hour + 4*time.Minute + 5*time.Second, false},

		// Errors
		{"", 0, true},
		{"d", 0, true},
		{"w", 0, true},
		{".s", 0, true},
		{"1x", 0, true},
		{"1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && time.Duration(got) != tt.expected {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
