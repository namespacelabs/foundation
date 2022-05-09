// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	srv := &schema.Server{
		Id:          "abc",
		PackageName: "namespacelabs.dev/foundation/test",
	}

	got := MakeAnnotations(&schema.Stack_Entry{Server: srv})

	if d := cmp.Diff(map[string]string{
		"k8s.namespacelabs.dev/server-package-name": "namespacelabs.dev/foundation/test",
	}, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
