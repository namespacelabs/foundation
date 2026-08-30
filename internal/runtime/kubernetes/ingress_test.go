// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"testing"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func TestBindNamespaceSkipsIngressControllerForEphemeralNscloud(t *testing.T) {
	env := &schema.Environment{Name: "test", Ephemeral: true}
	workspace := cfg.MakeSyntheticWorkspace(&schema.Workspace{ModuleName: "example.com/test"}, nil)
	config := cfg.MakeConfigurationWith(env.Name, workspace, cfg.ConfigurationSlice{
		Configuration: protos.WrapAnysOrDie(&client.HostEnv{Provider: "nscloud"}),
	})

	bound := bindNamespace(cfg.MakeUnverifiedContext(config, env))
	if !bound.skipIngressControllerWait {
		t.Fatal("ephemeral nscloud namespace waits for an in-cluster ingress controller")
	}
}

func TestPlanIngressWaitsForControllerInEphemeralEnvironment(t *testing.T) {
	const packageName = "example.com/test/server"

	for _, tt := range []struct {
		name                      string
		skipIngressControllerWait bool
		wantReadiness             bool
	}{
		{name: "local", wantReadiness: true},
		{name: "nscloud", skipIngressControllerWait: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := planIngress(context.Background(), nginx.IngressClass(), BoundNamespace{
				env:                       &schema.Environment{Name: "test", Ephemeral: true},
				namespace:                 "test",
				skipIngressControllerWait: tt.skipIngressControllerWait,
			}, &schema.Stack{Entry: []*schema.Stack_Entry{{
				Server: &schema.Server{PackageName: packageName, Id: "server"},
			}}}, []*schema.IngressFragment{{
				Name:   "server",
				Owner:  packageName,
				Domain: &schema.Domain{Fqdn: "server.example.com"},
				HttpPath: []*schema.IngressFragment_IngressHttpPath{{
					Path: "/", Service: "server", ServicePort: 8080,
				}},
			}})
			if err != nil {
				t.Fatalf("planIngress: %v", err)
			}

			var found bool
			for _, def := range plan.Definitions {
				if def.GetImpl().MessageIs(&kubedef.OpEnsureIngressController{}) {
					found = true
					break
				}
			}
			if found != tt.wantReadiness {
				t.Fatalf("controller readiness operation = %t, want %t", found, tt.wantReadiness)
			}
		})
	}
}
