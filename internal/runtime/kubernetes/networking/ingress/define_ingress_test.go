// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ingress

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/schema"
)

func TestGroupByName(t *testing.T) {
	x := []*schema.IngressFragment{
		{Name: "a"},
		{Name: "b"},
		{Name: "a"},
	}

	got := groupByName(x)

	if d := cmp.Diff([]ingressGroup{
		{
			Name: "a",
			Fragments: []*schema.IngressFragment{
				{Name: "a"},
				{Name: "a"},
			},
		},
		{
			Name: "b",
			Fragments: []*schema.IngressFragment{
				{Name: "b"},
			},
		},
	}, got, protocmp.Transform()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
