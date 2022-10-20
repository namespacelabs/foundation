// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ingress

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/networking/dns"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

func Register() {
	RegisterRuntimeState()

	execution.RegisterFuncs(execution.Funcs[*OpMapAddress]{
		Handle: func(ctx context.Context, g *fnschema.SerializedInvocation, op *OpMapAddress) (*execution.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			return nil, tasks.Action("ingress.publish-address").Arg("fqdn", op.Fdqn).Run(ctx, func(ctx context.Context) error {
				ingressSvc := nginx.IngressLoadBalancerService() // Make nginx reference configurable.

				return waitForIngress(ctx, cluster.PreparedClient().Clientset, ingressSvc, op)
			})
		},

		PlanOrder: func(ctx context.Context, _ *OpMapAddress) (*fnschema.ScheduleOrder, error) {
			return &fnschema.ScheduleOrder{
				SchedAfterCategory: []string{kubedef.MakeSchedCat(schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"})},
			}, nil
		},
	})

	execution.RegisterFuncs(execution.Funcs[*OpCleanupMigration]{
		Handle: func(ctx context.Context, g *fnschema.SerializedInvocation, op *OpCleanupMigration) (*execution.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			return nil, tasks.Action("kubernetes.ingress.cleanup-migration").Run(ctx, func(ctx context.Context) error {
				ingresses, err := cluster.PreparedClient().Clientset.NetworkingV1().Ingresses(op.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
				})
				if err != nil {
					return fnerrors.Wrapf(nil, err, "unable to list ingresses")
				}

				// We no longer emit "-managed" ingresses.
				for _, ingress := range ingresses.Items {
					if strings.HasSuffix(ingress.Name, "-managed") {
						if err := cluster.PreparedClient().Clientset.NetworkingV1().Ingresses(op.Namespace).Delete(ctx, ingress.Name, metav1.DeleteOptions{}); err != nil {
							return err
						}
					}
				}

				return nil
			})
		},

		PlanOrder: func(ctx context.Context, _ *OpCleanupMigration) (*fnschema.ScheduleOrder, error) {
			return &fnschema.ScheduleOrder{
				SchedAfterCategory: []string{kubedef.MakeSchedCat(schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"})},
			}, nil
		},
	})

	nginx.RegisterGraphHandlers()
}

func waitForIngress(ctx context.Context, cli *k8s.Clientset, ingressSvc *nginx.NameRef, op *OpMapAddress) error {
	return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 1*time.Minute, func(ctx context.Context) (bool, error) {
		// If the ingress declares there's a load balancer service that backs itself, then look
		// for the LB address instead of waiting for the ingress to be mapped.
		if ingressSvc != nil {
			svc, err := cli.CoreV1().Services(ingressSvc.Namespace).Get(ctx, ingressSvc.ServiceName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				done, err := checkMap(ctx, svc.Status.LoadBalancer, op.Fdqn)
				if err != nil {
					return false, err
				} else if done {
					return true, nil
				}
			}
		}

		ingress, err := cli.NetworkingV1().Ingresses(op.IngressNs).Get(ctx, op.IngressName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		return checkMap(ctx, ingress.Status.LoadBalancer, op.Fdqn)
	})
}

func checkMap(ctx context.Context, lb corev1.LoadBalancerStatus, fqdn string) (bool, error) {
	addr, uptodate := loadBalancerAddress(ctx, lb, fqdn)
	if uptodate {
		return true, nil
	}

	if addr == "" {
		return false, nil
	}

	if err := fnapi.Map(ctx, fqdn, addr); err != nil {
		return false, err
	}

	return true, nil
}

func loadBalancerAddress(ctx context.Context, lb corev1.LoadBalancerStatus, fqdn string) (string, bool) {
	resolver := dns.Resolver{
		Timeout:     2 * time.Second,
		Nameservers: []string{"1.1.1.1:53"},
	}

	// XXX issue the lookup in parallel.
	rec, err := resolver.Lookup(fqdn)
	if err != nil {
		rec = nil
	}

	for _, endpoint := range lb.Ingress {
		if endpoint.IP != "" {
			// Check if already updated. Then nothing to do.
			return endpoint.IP, rec.A() == endpoint.IP
		} else if endpoint.Hostname != "" {
			// Check if already updated. Then nothing to do.
			return endpoint.Hostname, rec.CNAME() == endpoint.Hostname+"."
		}
	}

	return "", false
}
