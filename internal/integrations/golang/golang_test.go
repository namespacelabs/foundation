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

	endpoints, err := (impl{}).InternalEndpoints(
		&schema.Environment{Purpose: schema.Environment_TESTING},
		&schema.Server{PackageName: "example.com/server"},
		ports,
	)
	if err != nil {
		t.Fatalf("InternalEndpoints: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("got %d endpoints, want 1", len(endpoints))
	}
	if got := endpoints[0].Port.GetName(); got != "http-port" {
		t.Errorf("got port %q, want %q", got, "http-port")
	}
}
