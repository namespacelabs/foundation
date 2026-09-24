// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"testing"

	"gotest.tools/assert"
)

func TestProfilePlatformOS(t *testing.T) {
	for _, tt := range []struct {
		label, current, want string
	}{
		{"windows-2022", "linux", "windows"},
		{"windows-2022", "", "windows"},
		{"ubuntu-24.04", "windows", "linux"},
		{"ubuntu-22.04", "", "linux"},
		{"sequoia", "macos", "macos"},
	} {
		t.Run(tt.label+"/"+tt.current, func(t *testing.T) {
			assert.Equal(t, tt.want, profilePlatformOS(tt.label, tt.current))
		})
	}
}
