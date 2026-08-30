// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestNamespaceGenerator(t *testing.T) {
	w := &schema.Workspace{ModuleName: "namespacelabs.dev/foundation"}
	env := &schema.Environment{Name: "prod"}

	x := ModuleNamespace(w, env)
	expected := "prod-foundation-7l67u"

	if x != expected {
		t.Errorf("expected=%q, got=%q", expected, x)
	}
}

func TestServiceNames(t *testing.T) {
	for _, name := range []string{"Invalid", "", "test_foo", "may-not-end-with-dash-",
		"too-long-name-with-more-than-sixty-three-characters-should-be-rejected"} {
		if validateServiceName(name) == nil {
			t.Errorf("invalid service name %q should have been rejected", name)
		}
	}

	for _, name := range []string{"a", "valid", "an0th3r-valid-servic3"} {
		if err := validateServiceName(name); err != nil {
			t.Errorf("valid service name %q was rejected: %v", name, err)
		}
	}
}
