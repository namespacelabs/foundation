// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"testing"
)

func TestGRPCTarget(t *testing.T) {
	for _, test := range []struct {
		endpoint   string
		wantTarget string
		wantSecure bool
		wantError  bool
	}{
		{endpoint: "grpcs://executor.example:443", wantTarget: "executor.example:443", wantSecure: true},
		{endpoint: "grpc://cache.example:80", wantTarget: "cache.example:80", wantSecure: false},
		{endpoint: "cache.example:443", wantTarget: "cache.example:443", wantSecure: false},
		{endpoint: "grpcs://cache.example/%zz", wantError: true},
		{endpoint: "https://cache.example", wantError: true},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			gotTarget, gotSecure, err := grpcTarget(test.endpoint)
			if (err != nil) != test.wantError {
				t.Fatalf("grpcTarget(%q) error = %v, wantError = %t", test.endpoint, err, test.wantError)
			}
			if gotTarget != test.wantTarget || gotSecure != test.wantSecure {
				t.Fatalf("grpcTarget(%q) = (%q, %t), want (%q, %t)", test.endpoint, gotTarget, gotSecure, test.wantTarget, test.wantSecure)
			}
		})
	}
}
