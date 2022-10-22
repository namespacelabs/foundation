// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package grpc

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseConnMap(t *testing.T) {
	got := parseConn("foo/bar:foo/quux/quux.Service=quuxserver;;;foo/bar:foo/bar/bar.Service=barserver;")

	if d := cmp.Diff(map[string]string{
		"foo/bar:foo/bar/bar.Service":   "barserver",
		"foo/bar:foo/quux/quux.Service": "quuxserver",
	}, got); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
