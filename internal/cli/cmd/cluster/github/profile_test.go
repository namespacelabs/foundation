// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import "testing"

func TestProfilePlatformOS(t *testing.T) {
	tests := []struct {
		name      string
		label     string
		currentOS string
		want      string
	}{
		{name: "windows create", label: "windows-2022", currentOS: "linux", want: "windows"},
		{name: "windows update", label: "windows-2025", currentOS: "macos", want: "windows"},
		{name: "ubuntu create", label: "ubuntu-24.04", currentOS: "linux", want: "linux"},
		{name: "windows to ubuntu", label: "ubuntu-24.04", currentOS: "windows", want: "linux"},
		{name: "macos create", label: "goldengate", currentOS: "linux", want: "macos"},
		{name: "macos staging label", label: "sequoia-staging", currentOS: "linux", want: "macos"},
		{name: "unknown update", label: "future-os", currentOS: "macos", want: "macos"},
		{name: "unknown create", label: "future-os", want: "linux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := profilePlatformOS(tt.label, tt.currentOS); got != tt.want {
				t.Errorf("profilePlatformOS(%q, %q) = %q; want %q", tt.label, tt.currentOS, got, tt.want)
			}
		})
	}
}
