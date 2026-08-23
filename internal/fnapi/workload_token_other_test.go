// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

//go:build !windows

package fnapi

import "testing"

func TestDefaultWorkloadTokenPath(t *testing.T) {
	path, err := defaultWorkloadTokenPath()
	if err != nil {
		t.Fatal(err)
	}

	if path != "/var/run/nsc/token.json" {
		t.Fatalf("defaultWorkloadTokenPath() = %q, want %q", path, "/var/run/nsc/token.json")
	}
}
