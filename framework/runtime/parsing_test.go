// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/schema/runtime"
)

func TestLoadBuildVCSWithoutDeploymentConfig(t *testing.T) {
	if _, err := os.Stat("/namespace/config/buildvcs.json"); !os.IsNotExist(err) {
		t.Skip("requires an environment without deployment-injected BuildVCS")
	}

	gitOutput := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", args...).CombinedOutput()
		if !assert.NoError(t, err, "%s", out) {
			t.FailNow()
		}
		return strings.TrimSpace(string(out))
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("requires git to build a VCS-stamped binary")
	}
	if err := exec.Command("git", "rev-parse", "--git-dir").Run(); err != nil {
		t.Skip("requires a git checkout to build a VCS-stamped binary")
	}

	commitTime, err := strconv.ParseInt(gitOutput("show", "-s", "--format=%ct", "HEAD"), 10, 64)
	if !assert.NoError(t, err) {
		return
	}
	want := &runtime.BuildVCS{
		Revision:    gitOutput("rev-parse", "HEAD"),
		CommitTime:  time.Unix(commitTime, 0).UTC().Format(time.RFC3339),
		Uncommitted: gitOutput("status", "--porcelain") != "",
	}

	for _, mode := range []string{"true", "false"} {
		t.Run("buildvcs="+mode, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "buildvcs")
			build := exec.Command("go", "build", "-trimpath", "-buildvcs="+mode, "-o", binary, "./testdata/buildvcs")
			if out, err := build.CombinedOutput(); !assert.NoError(t, err, "%s", out) {
				return
			}
			out, err := exec.Command(binary).CombinedOutput()
			if !assert.NoError(t, err, "%s", out) {
				return
			}
			var got *runtime.BuildVCS
			if !assert.NoError(t, json.Unmarshal(out, &got)) {
				return
			}
			if mode == "true" {
				assert.Equal(t, want, got)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}
