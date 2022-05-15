// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
)

func (r k8sRuntime) PlanIngress(ctx context.Context, stack *schema.Stack, allFragments []*schema.IngressFragment) (runtime.DeploymentState, error) {
	var state deploymentState

	certSecretMap, secrets := ingress.MakeCertificateSecrets(r.moduleNamespace, allFragments)

	for _, apply := range secrets {
		// XXX we could actually collect which servers refer what certs, to use as scope.
		def, err := apply.ToDefinition()
		if err != nil {
			return nil, err
		}
		state.definitions = append(state.definitions, def)
	}

	// XXX ensure that any single domain is only used by a single ingress.
	var managed []ingress.MapAddress
	for _, srv := range stack.Entry {
		var frags []*schema.IngressFragment
		for _, fr := range allFragments {
			if srv.GetPackageName().Equals(fr.Owner) {
				frags = append(frags, fr)
			}
		}

		if len(frags) == 0 {
			continue
		}

		defs, m, err := ingress.Ensure(ctx, serverNamespace(r.boundEnv, srv.Server), r.env, srv.Server, frags, certSecretMap)
		if err != nil {
			return nil, err
		}

		for _, apply := range defs {
			def, err := apply.ToDefinition(srv.GetPackageName())
			if err != nil {
				return nil, err
			}
			state.definitions = append(state.definitions, def)
		}

		managed = append(managed, m...)
	}

	// XXX this could be reduced in effort (e.g. batched).
	for _, frag := range managed {
		impl, err := anypb.New(&ingress.OpMapAddress{
			Fdqn:        frag.FQDN,
			IngressNs:   frag.Ingress.Namespace,
			IngressName: frag.Ingress.Name,
		})
		if err != nil {
			return nil, err
		}

		state.definitions = append(state.definitions, &schema.Definition{
			Description: fmt.Sprintf("Update %s's address", frag.FQDN),
			Impl:        impl,
		})
	}

	return state, nil
}

func (r k8sRuntime) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f runtime.PortForwardedFunc) (io.Closer, error) {
	svc := nginx.IngressLoadBalancerService()
	// XXX watch?
	resolved, err := r.cli.CoreV1().Services(svc.Namespace).Get(ctx, svc.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pod, err := resolvePodByLabels(ctx, r.cli, io.Discard, svc.Namespace, resolved.Spec.Selector)
	if err != nil {
		return nil, err
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	go func() {
		if err := r.boundEnv.startAndBlockPortFwd(ctxWithCancel, fwdArgs{
			Namespace:     svc.Namespace,
			Identifier:    "ingress",
			LocalAddrs:    localAddrs,
			LocalPort:     localPort,
			ContainerPort: svc.ContainerPort,

			Watch: func(_ context.Context, f func(*v1.Pod, int64, error)) func() {
				f(&pod, 1, nil)
				return func() {}
			},
			ReportPorts: func(p runtime.ForwardedPort) {
				f(runtime.ForwardedPortEvent{
					Added: []runtime.ForwardedPort{{
						LocalPort:     p.LocalPort,
						ContainerPort: p.ContainerPort,
					}},
					Endpoint: &schema.Endpoint{
						ServiceName: runtime.IngressServiceName,
						ServiceMetadata: []*schema.ServiceMetadata{{
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
