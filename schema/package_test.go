// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package schema

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestParsePackageRef(t *testing.T) {
	for _, test := range []struct {
		Source   string
		Expected *PackageRef
	}{
		{"foobar/quux", &PackageRef{PackageName: "foobar/quux"}},
		{"foobar/quux:bar", &PackageRef{PackageName: "foobar/quux", Name: "bar"}},
	} {
		got, err := ParsePackageRef(test.Source)
		if err != nil {
			t.Error(err)
		} else {
			if d := cmp.Diff(test.Expected, got, protocmp.Transform()); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		}
	}
}
