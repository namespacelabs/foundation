// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/schema"
)

func TestEnsureInvocationOrder(t *testing.T) {
	handlers := []*tool.Definition{
		{TargetServer: "a", Source: tool.Source{DeclaredStack: []schema.PackageName{"a", "b", "c"}}},
		{TargetServer: "b", Source: tool.Source{DeclaredStack: []schema.PackageName{"b", "c"}}},
		{TargetServer: "c", Source: tool.Source{}},
	}

	perServer := map[schema.PackageName]*serverDefs{
		"a": {Ops: []*schema.SerializedInvocation{{Description: "a"}}},
		"b": {Ops: []*schema.SerializedInvocation{{Description: "b"}}},
		"c": {Ops: []*schema.SerializedInvocation{{Description: "c"}}},
	}

	got, err := ensureInvocationOrder(handlers, perServer)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff([]*schema.SerializedInvocation{
		{Description: "c"},
		{Description: "b"},
		{Description: "a"},
	}, got, protocmp.Transform()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
