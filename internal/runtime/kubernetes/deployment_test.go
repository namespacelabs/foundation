// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/client-go/applyconfigurations/apps/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

func TestDeploymentSidecarResources(t *testing.T) {
	for _, tt := range []struct {
		name             string
		requests, limits *schema.Container_ResourceLimits
		want             string
	}{
		{"inherited", nil, nil, `{"requests":{"cpu":"8","memory":"48Gi"},"limits":{"cpu":"16","memory":"64Gi"}}`},
		{"requests", &schema.Container_ResourceLimits{Cpu: "1", Memory: "4Gi"}, nil, `{"requests":{"cpu":"1","memory":"4Gi"},"limits":{"cpu":"16","memory":"64Gi"}}`},
		{"limits", nil, &schema.Container_ResourceLimits{Cpu: "2", Memory: "6Gi"}, `{"requests":{"cpu":"8","memory":"48Gi"},"limits":{"cpu":"2","memory":"6Gi"}}`},
		{"partial requests replace block", &schema.Container_ResourceLimits{Cpu: "1"}, &schema.Container_ResourceLimits{Cpu: "2", Memory: "6Gi"}, `{"requests":{"cpu":"1"},"limits":{"cpu":"2","memory":"6Gi"}}`},
		{"explicit empty", &schema.Container_ResourceLimits{}, &schema.Container_ResourceLimits{}, `{}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sidecar := runtime.SidecarRunOpts{
				Name: "vector", BinaryRef: schema.MakePackageSingleRef("example.com/vector"),
				ContainerRunOpts: runtime.ContainerRunOpts{
					Image:            oci.ImageID{Repository: "example.com/vector"},
					ResourceRequests: tt.requests, ResourceLimits: tt.limits,
				},
			}
			state := &serverRunState{}
			err := prepareDeployment(context.Background(), BoundNamespace{namespace: "test-ns"}, runtime.DeployableSpec{
				Id: "serverid", Name: "storage", Class: schema.DeployableClass_STATELESS,
				PackageRef: schema.MakePackageSingleRef("example.com/storage"),
				MainContainer: runtime.ContainerRunOpts{
					Image:            oci.ImageID{Repository: "example.com/storage"},
					ResourceRequests: &schema.Container_ResourceLimits{Cpu: "8", Memory: "48Gi"},
					ResourceLimits:   &schema.Container_ResourceLimits{Cpu: "16", Memory: "64Gi"},
				},
				Sidecars: []runtime.SidecarRunOpts{sidecar}, Inits: []runtime.SidecarRunOpts{sidecar},
			}, deployOpts{}, state)
			if !assert.NoError(t, err) {
				return
			}
			op, ok := state.operations[len(state.operations)-1].(kubedef.EnsureDeployment)
			if !assert.True(t, ok) {
				return
			}
			deployment, ok := op.Resource.(*appsv1.DeploymentApplyConfiguration)
			if !assert.True(t, ok) {
				return
			}
			pod := deployment.Spec.Template.Spec
			assert.Len(t, pod.Containers, 2)
			assert.Len(t, pod.InitContainers, 1)
			for _, ctr := range append(pod.Containers, pod.InitContainers...) {
				encoded, err := json.Marshal(ctr.Resources)
				assert.NoError(t, err)
				want := tt.want
				if *ctr.Name == "storage" {
					want = `{"requests":{"cpu":"8","memory":"48Gi"},"limits":{"cpu":"16","memory":"64Gi"}}`
				}
				assert.JSONEq(t, want, string(encoded), *ctr.Name)
			}
		})
	}
}

func TestDeploymentStartupProbe(t *testing.T) {
	for _, env := range []struct {
		purpose                                 schema.Environment_Purpose
		period, failures, readinessInitialDelay int
	}{
		{schema.Environment_DEVELOPMENT, 1, 3, 1},
		{schema.Environment_TESTING, 1, 3, 1},
		{schema.Environment_PRODUCTION, 3, 5, 3},
	} {
		for _, tt := range []struct {
			name    string
			live    *schema.Probe
			handler string
		}{
			{"http", &schema.Probe{Kind: runtime.FnServiceLivez, Http: &schema.Probe_Http{Path: "/livez", ContainerPort: 8080}}, `"httpGet":{"path":"/livez","port":8080}`},
			{"exec", &schema.Probe{Kind: runtime.FnServiceLivez, Exec: &schema.Probe_Exec{Command: []string{"/bin/check", "live"}}}, `"exec":{"command":["/bin/check","live"]}`},
			{"readiness only", nil, ""},
		} {
			t.Run(env.purpose.String()+"/"+tt.name, func(t *testing.T) {
				probes := []*schema.Probe{{Kind: runtime.FnServiceReadyz, Http: &schema.Probe_Http{Path: "/readyz", ContainerPort: 9090}}}
				if tt.live != nil {
					probes = append(probes, tt.live)
				}
				state := &serverRunState{}
				err := prepareDeployment(context.Background(), BoundNamespace{
					namespace: "test-ns", env: &schema.Environment{Purpose: env.purpose},
				}, runtime.DeployableSpec{
					Id: "serverid", Name: "server", Class: schema.DeployableClass_STATELESS,
					PackageRef:    schema.MakePackageSingleRef("example.com/server"),
					MainContainer: runtime.ContainerRunOpts{Image: oci.ImageID{Repository: "example.com/server"}},
					Probes:        probes,
				}, deployOpts{platforms: []specs.Platform{{OS: "linux", Architecture: "amd64"}}}, state)
				if !assert.NoError(t, err) {
					return
				}
				op, ok := state.operations[len(state.operations)-1].(kubedef.EnsureDeployment)
				if !assert.True(t, ok) {
					return
				}
				var pod *applycorev1.PodSpecApplyConfiguration
				switch resource := op.Resource.(type) {
				case *applycorev1.PodApplyConfiguration:
					pod = resource.Spec
				case *appsv1.DeploymentApplyConfiguration:
					pod = resource.Spec.Template.Spec
				default:
					t.Fatalf("unexpected resource %T", resource)
				}
				if !assert.Len(t, pod.Containers, 1) {
					return
				}
				ctr := pod.Containers[0]
				readiness, err := json.Marshal(ctr.ReadinessProbe)
				assert.NoError(t, err)
				assert.JSONEq(t, fmt.Sprintf(`{"httpGet":{"path":"/readyz","port":9090},"initialDelaySeconds":%d,"periodSeconds":%d,"timeoutSeconds":1,"failureThreshold":%d}`, env.readinessInitialDelay, env.period, env.failures), string(readiness))
				if tt.live == nil {
					assert.Nil(t, ctr.StartupProbe)
					assert.Nil(t, ctr.LivenessProbe)
					return
				}
				startup, err := json.Marshal(ctr.StartupProbe)
				assert.NoError(t, err)
				if env.purpose == schema.Environment_TESTING {
					assert.JSONEq(t, `{`+tt.handler+`,"periodSeconds":1,"timeoutSeconds":1,"failureThreshold":30}`, string(startup))
				} else {
					assert.Nil(t, ctr.StartupProbe)
				}
				liveness, err := json.Marshal(ctr.LivenessProbe)
				assert.NoError(t, err)
				assert.JSONEq(t, fmt.Sprintf(`{%s,"initialDelaySeconds":1,"periodSeconds":%d,"timeoutSeconds":1,"failureThreshold":%d}`, tt.handler, env.period, env.failures), string(liveness))
			})
		}
	}
}

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

func TestDeploymentPreferredNodeAffinity(t *testing.T) {
	for _, tt := range []struct {
		name     string
		selector map[string]string
		want     string
	}{
		{"default", nil, `[]`},
		{"empty", map[string]string{}, `[]`},
		{"preferred labels", map[string]string{"example.com/bandwidth": "200", "example.com/rack": "a"}, `[
			{"weight":100,"preference":{"matchExpressions":[
				{"key":"example.com/bandwidth","operator":"In","values":["200"]},
				{"key":"example.com/rack","operator":"In","values":["a"]}
			]}}
		]`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := &serverRunState{}
			err := prepareDeployment(context.Background(), BoundNamespace{namespace: "test-ns"}, runtime.DeployableSpec{
				Id: "serverid", Name: "server", Class: schema.DeployableClass_STATELESS,
				PackageRef:            schema.MakePackageSingleRef("example.com/server"),
				MainContainer:         runtime.ContainerRunOpts{Image: oci.ImageID{Repository: "example.com/server"}},
				NodeSelector:          []*schema.NodeSelectorItem{{Key: "example.com/pool", Value: "services"}},
				PodAntiAffinity:       &schema.PodAntiAffinity{TopologyKey: "kubernetes.io/hostname"},
				PreferredNodeAffinity: tt.selector,
			}, deployOpts{platforms: []specs.Platform{{Architecture: "arm64"}}}, state)
			if !assert.NoError(t, err) {
				return
			}
			op, ok := state.operations[len(state.operations)-1].(kubedef.EnsureDeployment)
			if !assert.True(t, ok) {
				return
			}
			deployment, ok := op.Resource.(*appsv1.DeploymentApplyConfiguration)
			if !assert.True(t, ok) {
				return
			}
			pod := deployment.Spec.Template.Spec
			assert.Equal(t, map[string]string{"example.com/pool": "services"}, pod.NodeSelector)
			encoded, err := json.Marshal(pod.Affinity)
			assert.NoError(t, err)
			var affinity map[string]json.RawMessage
			if !assert.NoError(t, json.Unmarshal(encoded, &affinity)) {
				return
			}
			want := `{"requiredDuringSchedulingIgnoredDuringExecution":{"nodeSelectorTerms":[{"matchExpressions":[{"key":"kubernetes.io/arch","operator":"In","values":["arm64"]}]}]}}`
			if len(tt.selector) > 0 {
				want = want[:len(want)-1] + `,"preferredDuringSchedulingIgnoredDuringExecution":` + tt.want + `}`
			}
			assert.JSONEq(t, want, string(affinity["nodeAffinity"]))
			assert.JSONEq(t, `{"requiredDuringSchedulingIgnoredDuringExecution":[{"labelSelector":{"matchLabels":{"k8s.namespacelabs.dev/server-package-name":"example-com-server"}},"topologyKey":"kubernetes.io/hostname"}]}`, string(affinity["podAntiAffinity"]))
		})
	}
}
