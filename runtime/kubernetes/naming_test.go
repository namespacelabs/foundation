// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestNamespaceGenerator(t *testing.T) {
	w := &schema.Workspace{ModuleName: "namespacelabs.dev/foundation"}
	env := &schema.Environment{Name: "prod"}

	x := namespace(w, env)
	expected := "prod-foundation-7l67u"

	if x != expected {
		t.Errorf("expected=%q, got=%q", expected, x)
	}
}