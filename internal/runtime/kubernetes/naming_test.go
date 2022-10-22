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
