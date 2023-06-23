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
	namespace := cmd.Flags().String("namespace", "default", "Namespace of the service load balancer to export.")
	service := cmd.Flags().String("service", "", "Name of the service load balancer to export.")
	port := cmd.Flags().Int32("port", 0, "Which port of service load balancer to export.")

	ip := cmd.Flags().String("ip", "", "Which ip of service load balancer to export with the ingress.")
	_ = cmd.Flags().MarkHidden("ip")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := selectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		if *service == "" {
			return fnerrors.New("--service is required")
		}

		backend, err := selectBackend(ctx, cluster, *namespace, *service, *port, *ip)
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
			backend.Port, *namespace, *service, resp.Fqdn)

		return nil
	})

	return cmd
}

func selectBackend(ctx context.Context, cluster *api.KubernetesCluster, ns, service string, port int32, ipAddr string) (*api.IngressBackendEndpoint, error) {
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

		port, err := selectPort(svc, port)
		if err != nil {
			return nil, err
		}

		ipAddr, err := selectIpAddr(svc, ipAddr)
		if err != nil {
			return nil, err
		}

		return &api.IngressBackendEndpoint{
			IpAddr: ipAddr,
			Port:   port,
		}, nil
	})
}

func selectPort(svc *corev1.Service, port int32) (int32, error) {
	var ports []int32
	for _, port := range svc.Spec.Ports {
		ports = append(ports, port.Port)
	}

	if port == 0 {
		switch len(ports) {
		case 0:
			return 0, fnerrors.New("service %q exposes no ports", svc.Name)
		case 1:
			port = ports[0]
		default:
			return 0, fnerrors.New("Service %q exports multiple ports %v. Please select one with --port.", svc.Name, ports)
		}
	}

	if !slices.Contains(ports, port) {
		return 0, fnerrors.New("service %q does not export port %d. Found ports: %v.", svc.Name, port, ports)
	}

	return port, nil
}

func selectIpAddr(svc *corev1.Service, ipAddr string) (string, error) {
	var ipAddrs []string
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP == "" {
			continue
		}

		ipAddrs = append(ipAddrs, ingress.IP)
	}

	if ipAddr == "" {
		switch len(ipAddrs) {
		case 0:
			return "", fnerrors.New("service %q has no exported ip addresses. This is unexpected. Please contact support@namespacelabs.com.", svc.Name)
		case 1:
			return ipAddrs[0], nil
		default:
			return "", fnerrors.New("Service %q has multiple exported ip addresses %v. Please select one with --ip.", svc.Name, ipAddrs)
		}
	}

	if !slices.Contains(ipAddrs, ipAddr) {
		return "", fnerrors.New("service %q is not exported on ip address %s. Found addresses: %v.", svc.Name, ipAddr, ipAddrs)
	}

	return ipAddr, nil
}
