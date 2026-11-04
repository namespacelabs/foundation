// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestBazelBinaryPlan(t *testing.T) {
	for _, tt := range []struct {
		name, plan, wantErr string
	}{
		{"list", `build_plan: [{bazel_build: {target: "//images:server", platforms: {"linux/arm64": "//platforms:arm64"}}}]`, ""},
		{"object", `build_plan: layer_build_plan: [{bazel_build: {target: "//images:server", platforms: {"linux/arm64": "//platforms:arm64"}}}]`, ""},
		{"shorthand", `from: bazel_build: {target: "//images:server", platforms: {"linux/arm64": "//platforms:arm64"}}`, ""},
		{"exclusive", `from: {bazel_build: {target: "//images:server"}, dockerfile: "Dockerfile"}`, "exactly one instruction"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &fncue.CueV{Val: cuecontext.New().CompileString(`{name: "server", ` + tt.plan + `}`)}
			bin, err := parseCueBinary(context.Background(), nil, pkggraph.Location{}, v, v)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			if !assert.NoError(t, err) || !assert.Len(t, bin.BuildPlan.LayerBuildPlan, 1) {
				return
			}
			plan := bin.BuildPlan.LayerBuildPlan[0].BazelBuild
			if assert.NotNil(t, plan) {
				assert.Equal(t, "//images:server", plan.Target)
				assert.Equal(t, map[string]string{"linux/arm64": "//platforms:arm64"}, plan.Platforms)
			}
		})
	}
}
