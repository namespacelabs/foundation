// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestConsolidat(t *testing.T) {
	x := &schema.Binary{Name: "abc"}

	var y *schema.Binary

	if !CheckConsolidate(x, &y) {
		t.Fatal("expected consolidation to work")
	}

	if y.GetName() != "abc" {
		t.Fatal("expected instance to have been updated")
	}

	if !CheckConsolidate(&schema.Binary{Name: "abc"}, &y) {
		t.Fatal("expected consolidation to work")
	}

	if CheckConsolidate(&schema.Binary{Name: "foobar"}, &y) {
		t.Fatal("expected consolidation to fail")
	}
}
