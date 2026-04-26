// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"testing"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

func TestDeployEndpointAppliesLoadBalancerClass(t *testing.T) {
	state := &serverRunState{}

	err := deployEndpoint(context.Background(), BoundNamespace{namespace: "test-ns"}, runtime.DeployableSpec{
		Id:         "serverid",
		Name:       "server",
		PackageRef: &schema.PackageRef{PackageName: "example.com/server"},
	}, &schema.Endpoint{
		Type:              schema.Endpoint_LOAD_BALANCER,
		ServiceName:       "rawlistener",
		AllocatedName:     "rawlistener",
		LoadBalancerClass: "tailscale",
		Ports: []*schema.Endpoint_PortMap{{
			ExportedPort: 8080,
			Port: &schema.Endpoint_Port{
				Name:          "server-port",
				ContainerPort: 8080,
			},
		}},
	}, state)
	if err != nil {
		t.Fatalf("deployEndpoint failed: %v", err)
	}

	if len(state.operations) != 1 {
		t.Fatalf("expected one operation, got %d", len(state.operations))
	}

	op, ok := state.operations[0].(kubedef.Apply)
	if !ok {
		t.Fatalf("expected kubedef.Apply, got %T", state.operations[0])
	}

	svc, ok := op.Resource.(*applycorev1.ServiceApplyConfiguration)
	if !ok {
		t.Fatalf("expected ServiceApplyConfiguration, got %T", op.Resource)
	}

	if svc.Spec == nil || svc.Spec.LoadBalancerClass == nil {
		t.Fatalf("expected loadBalancerClass to be set")
	}

	if got := *svc.Spec.LoadBalancerClass; got != "tailscale" {
		t.Fatalf("expected loadBalancerClass %q, got %q", "tailscale", got)
	}
}
