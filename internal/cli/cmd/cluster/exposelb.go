// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewExposeLoadBalancerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "expose-lb [cluster-id]",
		Aliases: []string{"expose-load-balancer"},
		Short:   "Opens a public ingress to the specified service load balancer.",
		Args:    cobra.MaximumNArgs(1),
	}

	name := cmd.Flags().String("name", "", "If specified, set the name of the exposed ingress. Only permitted when exposing a single port. By default, ingress names are generated.")
	namespace := cmd.Flags().StringP("namespace", "ns", "default", "Namespace of the service load balancer to export.")
	service := cmd.Flags().String("service", "", "Name of the service load balancer to export.")
	port := cmd.Flags().Int32("service", 0, "Which port of service load balancer to export.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := selectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		backend, err := selectBackend(ctx, cluster, *namespace, *service, *port)
		if err != nil {
			return err
		}

		resp, err := api.RegisterIngress(ctx, api.Methods, api.RegisterIngressRequest{
			ClusterId:       cluster.ClusterId,
			Name:            *name,
			BackendEndpoint: backend,
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "Exported port %d from %s/%s:\n  https://%s\n\n",
			*port, *namespace, *service, resp.Fqdn)

		return nil
	})

	return cmd
}

func selectBackend(ctx context.Context, cluster *api.KubernetesCluster, ns, service string, port int32) (*api.IngressBackendEndpoint, error) {
	return tasks.Return(ctx, tasks.Action("nsc.expose-lb").HumanReadablef("Querying exported service load balancers"), func(ctx context.Context) (*api.IngressBackendEndpoint, error) {
		cfg := clientcmd.NewDefaultClientConfig(ctl.MakeConfig(cluster), nil)
		restcfg, err := cfg.ClientConfig()
		if err != nil {
			return nil, fnerrors.New("failed to load kubernetes configuration: %w", err)
		}

		cli, err := k8s.NewForConfig(restcfg)
		if err != nil {
			return nil, fnerrors.New("failed to create kubernetes client: %w", err)
		}

		svc, err := cli.CoreV1().Services(ns).Get(ctx, service, metav1.GetOptions{})
		if err != nil {
			return nil, fnerrors.InvocationError("kubernetes", "failed to query service %q: %w", service, err)
		}

		if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			return nil, fnerrors.New("service %q is not a load balancer (found type %q)", service, svc.Spec.Type)
		}

		var ports []int32
		for _, port := range svc.Spec.Ports {
			ports = append(ports, port.Port)
		}

		if !slices.Contains(ports, port) {
			return nil, fnerrors.New("Service %q does not export port %d. Found ports: %v.", service, port, ports)
		}

		return &api.IngressBackendEndpoint{
			IpAddr: svc.Spec.ClusterIP, // XXX should this be status.loadbalancer.ingress[0].ip?
			Port:   port,
		}, nil
	})
}
