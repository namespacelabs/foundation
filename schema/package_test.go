// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package schema

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

const owner = PackageName("owner/pkg")

func TestValidParsePackageRef(t *testing.T) {
	for _, test := range []struct {
		Source   string
		Expected *PackageRef
	}{
		{"foobar/quux", &PackageRef{PackageName: "foobar/quux"}},
		{"foobar/quux:bar", &PackageRef{PackageName: "foobar/quux", Name: "bar"}},
		{":baz", &PackageRef{PackageName: "owner/pkg", Name: "baz"}},
	} {
		got, err := ParsePackageRef(owner, test.Source)
		if err != nil {
			t.Error(err)
		} else {
			if d := cmp.Diff(test.Expected, got, protocmp.Transform()); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		}
	}
}

func TestInvalidParsePackageRef(t *testing.T) {
	// Invalid refs
	for _, ref := range []string{"", "::", "example.com/path:name:another"} {
		if _, err := ParsePackageRef(owner, ref); err == nil {
			t.Errorf("package ref %q should not be valid", ref)
		}
	}
}
