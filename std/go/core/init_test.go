// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package core

import "testing"

func TestEnforceBefore(t *testing.T) {
	initializers := []*Initializer{
		{Package: pkg("a")},
		{Package: pkg("b"), Before: []string{"a"}},
		{Package: pkg("c"), After: []string{"e"}},
		{Package: pkg("d"), Before: []string{"b"}},
		{Package: pkg("e")},
	}

	if _, err := enforceOrder(initializers); err != nil {
		t.Fatal(err)
	}
}

func pkg(name string) *Package { return &Package{PackageName: name} }
