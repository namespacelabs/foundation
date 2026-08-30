// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"testing"

	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestParseProbes(t *testing.T) {
	if _, err := parseProbes(
		pkggraph.Location{},
		[]*schema.Probe{
			{Kind: runtime.FnServiceReadyz, Http: &schema.Probe_Http{Path: "/"}},
			{Kind: runtime.FnServiceReadyz, Exec: &schema.Probe_Exec{Command: []string{"foo"}}},
		},
		cueServerExtension{},
	); err == nil {
		t.Fatalf("expected parse to fail")
	}

	if _, err := parseProbes(
		pkggraph.Location{},
		[]*schema.Probe{
			{Kind: runtime.FnServiceReadyz, Http: &schema.Probe_Http{Path: "/"}},
		},
		cueServerExtension{
			ReadinessProbe: &cueProbe{Exec: &cueExecProbe{Command: []string{"foo"}}},
		},
	); err == nil {
		t.Fatalf("expected parse to fail")
	}

	if _, err := parseProbes(
		pkggraph.Location{},
		[]*schema.Probe{
			{Kind: runtime.FnServiceReadyz, Http: &schema.Probe_Http{Path: "/"}},
		},
		cueServerExtension{
			Probes: map[string]cueProbe{
				"readiness": {Exec: &cueExecProbe{Command: []string{"foo"}}},
			},
		},
	); err == nil {
		t.Fatalf("expected parse to fail")
	}

	if _, err := parseProbes(
		pkggraph.Location{},
		[]*schema.Probe{},
		cueServerExtension{
			ReadinessProbe: &cueProbe{Exec: &cueExecProbe{Command: []string{"foo"}}},
			Probes: map[string]cueProbe{
				"liveness": {Exec: &cueExecProbe{Command: []string{"foo"}}},
			},
		},
	); err == nil {
		t.Fatalf("expected parse to fail")
	}

	if _, err := parseProbes(
		pkggraph.Location{},
		[]*schema.Probe{
			{Kind: runtime.FnServiceReadyz, Http: &schema.Probe_Http{Path: "/"}},
		},
		cueServerExtension{
			Probes: map[string]cueProbe{
				"liveness": {Exec: &cueExecProbe{Command: []string{"foo"}}},
			},
		},
	); err != nil {
		t.Fatalf("parsing failed: %v", err)
	}
}
