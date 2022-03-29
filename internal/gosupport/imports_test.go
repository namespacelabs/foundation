// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gosupport

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestImports(t *testing.T) {
	x := NewGoImports("foobar")
	x.AddOrGet("foobar/quux")
	x.AddOrGet("google.golang.org/grpc")
	x.AddOrGet("namespacelabs.dev/foundation/std/go/grpc")
	x.AddOrGet("namespacelabs.dev/foundation/std/server/tracing")
	x.AddOrGet("superduper/grpc")

	x.ImportMap()

	if d := cmp.Diff([]singleImport{
		{TypeURL: "foobar/quux"},
		{TypeURL: "google.golang.org/grpc"},
		{Rename: "fngrpc", TypeURL: "namespacelabs.dev/foundation/std/go/grpc"},
		{TypeURL: "namespacelabs.dev/foundation/std/server/tracing"},
		{Rename: "grpc1", TypeURL: "superduper/grpc"},
	}, x.ImportMap()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}

}