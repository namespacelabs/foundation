// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	fnschema "namespacelabs.dev/foundation/schema"
)

func planIngress(ctx context.Context, r clusterTarget, stack *fnschema.Stack, allFragments []*fnschema.IngressFragment) (*runtime.DeploymentPlan, error) {
	var state runtime.DeploymentPlan

	cleanup, err := anypb.New(&ingress.OpCleanupMigration{Namespace: r.namespace})
	if err != nil {
		return nil, err
	}

	state.Definitions = append(state.Definitions, &fnschema.SerializedInvocation{
		Description: "Ingress migration cleanup",
		Impl:        cleanup,
	})

	certSecretMap, secrets := ingress.MakeCertificateSecrets(r.namespace, allFragments)

	for _, apply := range secrets {
		// XXX we could actually collect which servers refer what certs, to use as scope.
		def, err := apply.ToDefinition()
		if err != nil {
			return nil, err
		}
		state.Definitions = append(state.Definitions, def)
	}

	// XXX ensure that any single domain is only used by a single ingress.
	var allManaged ingress.MapAddressList
	for _, srv := range stack.Entry {
		var frags []*fnschema.IngressFragment
		for _, fr := range allFragments {
			if srv.GetPackageName().Equals(fr.Owner) {
				frags = append(frags, fr)
			}
		}

		if len(frags) == 0 {
			continue
		}

		defs, managed, err := ingress.Ensure(ctx, r.namespace, r.env, srv.Server, frags, certSecretMap)
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

		if err := allManaged.Merge(managed); err != nil {
			return nil, err
		}
	}

	// XXX this could be reduced in effort (e.g. batched).
	for _, frag := range allManaged.Sorted() {
		impl, err := anypb.New(&ingress.OpMapAddress{
			Fdqn:        frag.FQDN,
			IngressNs:   frag.Ingress.Namespace,
			IngressName: frag.Ingress.Name,
		})
		if err != nil {
			return nil, err
		}

		state.Definitions = append(state.Definitions, &fnschema.SerializedInvocation{
			Description: fmt.Sprintf("Update %s's address", frag.FQDN),
			Impl:        impl,
		})
	}

	return &state, nil
}

func (r *Cluster) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, notify runtime.PortForwardedFunc) (io.Closer, error) {
	svc := nginx.IngressLoadBalancerService()

	ctxWithCancel, cancel := context.WithCancel(ctx)
	obs := kubeobserver.NewPodObserver(ctxWithCancel, r.cli, svc.Namespace, nginx.ControllerSelector())

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
						ServiceName: runtime.IngressServiceName,
						ServiceMetadata: []*fnschema.ServiceMetadata{{
							Protocol: "http",
							Kind:     runtime.IngressServiceKind,
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
