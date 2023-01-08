// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/constants"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
)

func planIngress(ctx context.Context, ingressPlanner kubedef.IngressClass, r clusterTarget, stack *fnschema.Stack, allFragments []*fnschema.IngressFragment) (*runtime.DeploymentPlan, error) {
	var state runtime.DeploymentPlan

	for _, srv := range stack.Entry {
		frags := planning.IngressOwnedBy(allFragments, srv.GetPackageName())
		if len(frags) == 0 {
			continue
		}

		defs, err := ingress.PlanIngress(ctx, ingressPlanner, r.namespace, r.env, srv.Server, frags)
		if err != nil {
			return nil, err
		}

		for _, apply := range defs {
			def, err := apply.ToDefinition(srv.GetPackageName())
			if err != nil {
				return nil, err
			}
			state.Definitions = append(state.Definitions, def)
		}
	}

	// On ephemeral environments, e.g. tests, we don't wait for an
	// ingress controller to be present, before installing ingress
	// objects. This is because we sometimes run in environments where
	// there's no controller installed (e.g. in ephemeral nscloud
	// clusters). And tests don't (yet) exercise ingress objects.
	if len(state.Definitions) > 0 && !r.env.Ephemeral {
		var d defs.DefList

		d.AddExt("Ensure Ingress Controller", &kubedef.OpEnsureIngressController{},
			defs.Category(ingress.IngressControllerCat),
			// Lets make sure that we verify the controller after deployments and services are in place.
			defs.DependsOn(kubedef.MakeSchedCat(schema.GroupKind{Kind: "Service"})),
			// OpEnsureIngressController was introduced in orchestrator 11.
			defs.MinimumVersion(11))

		if serialized, err := d.Serialize(); err != nil {
			return nil, err
		} else {
			state.Definitions = append(state.Definitions, serialized...)
		}
	}

	return &state, nil
}

func (r *Cluster) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, notify runtime.PortForwardedFunc) (io.Closer, error) {
	ingress := r.Ingress()
	if ingress == nil {
		return nil, nil
	}

	svc := ingress.Service()
	if svc == nil {
		return nil, nil
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	obs := kubeobserver.NewPodObserver(ctxWithCancel, r.cli, svc.Namespace, svc.PodSelector)

	go func() {
		if err := r.StartAndBlockPortFwd(ctxWithCancel, StartAndBlockPortFwdArgs{
			Namespace:     svc.Namespace,
			Identifier:    "ingress",
			LocalAddrs:    localAddrs,
			LocalPort:     localPort,
			ContainerPort: svc.ContainerPort,
			PodResolver:   obs,
			ReportPorts: func(p runtime.ForwardedPort) {
				notify(runtime.ForwardedPortEvent{
					Added: []runtime.ForwardedPort{{
						LocalPort:     p.LocalPort,
						ContainerPort: p.ContainerPort,
					}},
					Endpoint: &fnschema.Endpoint{
						ServiceName: constants.IngressServiceName,
						ServiceMetadata: []*fnschema.ServiceMetadata{{
							Protocol: "http",
							Kind:     constants.IngressServiceKind,
						}},
					},
				})
			},
		}); err != nil {
			fmt.Fprintf(console.Errors(ctx), "ingress forwarding failed: %v\n", err)
		}
	}()

	return closerCallback(cancel), nil
}

type closerCallback func()

func (c closerCallback) Close() error {
	c()
	return nil
}
