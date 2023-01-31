// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	execution.RegisterFuncs(execution.Funcs[*kubedef.OpMapAddress]{
		Handle: func(ctx context.Context, g *fnschema.SerializedInvocation, op *kubedef.OpMapAddress) (*execution.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			return nil, tasks.Action("ingress.publish-address").Arg("fqdn", op.Fdqn).Run(ctx, func(ctx context.Context) error {
				return waitAndMap(ctx, cluster, op)
			})
		},

		PlanOrder: func(ctx context.Context, _ *kubedef.OpMapAddress) (*fnschema.ScheduleOrder, error) {
			return &fnschema.ScheduleOrder{
				SchedAfterCategory: []string{kubedef.MakeSchedCat(schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"})},
			}, nil
		},
	})

	// XXX deprecated, remove soon.
	execution.RegisterFuncs(execution.Funcs[*kubedef.OpCleanupMigration]{
		Handle: func(ctx context.Context, g *fnschema.SerializedInvocation, op *kubedef.OpCleanupMigration) (*execution.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			return nil, tasks.Action("kubernetes.ingress.cleanup-migration").Run(ctx, func(ctx context.Context) error {
				ingresses, err := cluster.PreparedClient().Clientset.NetworkingV1().Ingresses(op.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: kubedef.SerializeSelector(kubedef.ManagedByUs()),
				})
				if err != nil {
					return fnerrors.InvocationError("kubernetes", "unable to list ingresses: %w", err)
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

		PlanOrder: func(ctx context.Context, _ *kubedef.OpCleanupMigration) (*fnschema.ScheduleOrder, error) {
			return &fnschema.ScheduleOrder{
				SchedAfterCategory: []string{kubedef.MakeSchedCat(schema.GroupKind{Group: "networking.k8s.io", Kind: "Ingress"})},
			}, nil
		},
	})

	execution.RegisterHandlerFunc(func(ctx context.Context, _ *fnschema.SerializedInvocation, op *kubedef.OpEnsureIngressController) (*execution.HandleResult, error) {
		cluster, err := kubedef.InjectedKubeCluster(ctx)
		if err != nil {
			return nil, err
		}

		if _, err := EnsureState(ctx, cluster, op.IngressClass); err != nil {
			return nil, err
		}

		return nil, nil
	})

	nginx.RegisterGraphHandlers()
}

func waitAndMap(ctx context.Context, cluster kubedef.KubeCluster, op *kubedef.OpMapAddress) error {
	if op.CnameTarget != "" {
		return fnapi.Map(ctx, op.Fdqn, op.CnameTarget)
	}

	cli := cluster.PreparedClient().Clientset

	return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 1*time.Minute, func(ctx context.Context) (bool, error) {
		// If the ingress declares there's a load balancer service that backs itself, then look
		// for the LB address instead of waiting for the ingress to be mapped.
		if op.IngressService != nil {
			svc, err := cli.CoreV1().Services(op.IngressService.Namespace).Get(ctx, op.IngressService.ServiceName, metav1.GetOptions{})
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

		return checkMap(ctx, asCoreLBStatus(ingress.Status.LoadBalancer), op.Fdqn)
	})
}

func asCoreLBStatus(lb netv1.IngressLoadBalancerStatus) corev1.LoadBalancerStatus {
	var status corev1.LoadBalancerStatus
	for _, y := range lb.Ingress {
		status.Ingress = append(status.Ingress, corev1.LoadBalancerIngress{
			IP:       y.IP,
			Hostname: y.Hostname,
		})
	}
	return status
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
