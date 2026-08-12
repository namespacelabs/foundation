// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package golang

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestInternalEndpointsSelectsHealthPort(t *testing.T) {
	ports := []*schema.Endpoint_Port{
		{Name: "server-port", ContainerPort: 4000},
		{Name: "http-port", ContainerPort: 4001},
	}

	for _, tc := range []struct {
		name     string
		purpose  schema.Environment_Purpose
		wantPort string
	}{
		{name: "testing", purpose: schema.Environment_TESTING, wantPort: "http-port"},
		{name: "production", purpose: schema.Environment_PRODUCTION, wantPort: "http-port"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoints, err := (impl{}).InternalEndpoints(
				&schema.Environment{Purpose: tc.purpose},
				&schema.Server{PackageName: "example.com/server"},
				ports,
			)
			if err != nil {
				t.Fatalf("InternalEndpoints: %v", err)
			}
			if len(endpoints) != 1 {
				t.Fatalf("got %d endpoints, want 1", len(endpoints))
			}
			if got := endpoints[0].Port.GetName(); got != tc.wantPort {
				t.Errorf("got port %q, want %q", got, tc.wantPort)
			}
		})
	}
}
