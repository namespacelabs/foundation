// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"namespacelabs.dev/foundation/schema"
)

func TestMakeLabels(t *testing.T) {
	env := &schema.Environment{Name: "prod", Purpose: schema.Environment_PRODUCTION}

	srv := &schema.Server{
		Id:          "abc",
		PackageName: "namespacelabs.dev/foundation/test",
	}

	got := MakeLabels(env, srv)

	if d := cmp.Diff(map[string]string{
		"app.kubernetes.io/managed-by":        "foundation.namespace.so",
		"k8s.namespacelabs.dev/env":           "prod",
		"k8s.namespacelabs.dev/env-ephemeral": "false",
		"k8s.namespacelabs.dev/env-purpose":   "production",
		"k8s.namespacelabs.dev/server-id":     "abc",
	}, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}

func TestMakeAnnotations(t *testing.T) {
	env := &schema.Environment{Ephemeral: true}

	got := MakeAnnotations(env, "namespacelabs.dev/foundation/test")

	if d := cmp.Diff(map[string]string{
		"k8s.namespacelabs.dev/server-package-name": "namespacelabs.dev/foundation/test",
		"k8s.namespacelabs.dev/planner-version":     "1",
		"k8s.namespacelabs.dev/env-timeout":         defaultEphemeralTimeout.String(),
	}, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
