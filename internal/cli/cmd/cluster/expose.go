// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/nerdctl/pkg/labels"
	"github.com/containerd/nerdctl/pkg/portutil"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/ctl"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewExposeCmd() *cobra.Command {
	// `nsc expose` acts as an alias for `nsc expose container`
	cmd := newExposeContainerCmd("expose [instance-id]", true)

	cmd.AddCommand(newExposeContainerCmd("container [instance-id]", false))
	cmd.AddCommand(newExposeKubernetesCmd())

	return cmd
}

func newExposeContainerCmd(use string, hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    use,
		Short:  "Opens a public ingress to the specified exported port.",
		Args:   cobra.MaximumNArgs(1),
		Hidden: hidden,
	}

	source := cmd.Flags().String("source", "", "Where to lookup the container.")
	containerName := cmd.Flags().String("container", "", "Which container to export.")
	containerPorts := cmd.Flags().IntSlice("container_port", nil, "If specified, only exposes the specified ports.")
	name := cmd.Flags().String("name", "", "If specified, set the name of the exposed ingress. Only permitted when exposing a single port. By default, ingress names are generated.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	all := cmd.Flags().Bool("all", false, "If set to true, exports one ingress for each exported port of each running container.")
	ingressRules := cmd.Flags().StringToString("ingress", map[string]string{}, "Specify ingress rules for ports; specify * to apply rules to any port; separate each rule with ;.")
	wildcard := cmd.Flags().Bool("wildcard", false, "If set, generate a wildcard ingress for the exposed container port. Can only be used when exposing a single port.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *containerName == "" && !*all {
			return fnerrors.New("one of --all or --container is required")
		} else if *containerName != "" && *all {
			return fnerrors.New("only one of --all or --container may be specified")
		}

		if hidden && *output == "plain" {
			out := console.Warnings(ctx)
			fmt.Fprintf(out, "`nsc %s` is deprecated. For backwards compatibility it behaves as `nsc expose container [instance-id]`.\n", use)
			fmt.Fprintln(out, "To expose an exported port on a running container `nsc expose container [instance-id].")
			fmt.Fprintln(out, "To exponse a Kubernentes Load Balancer use `nsc expose kubernetes [instance-id]`.")
		}

		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		ports, err := selectPorts(ctx, cluster, *source, containerFilter{*all, *containerName})
		if err != nil {
			return err
		}

		if len(*containerPorts) > 0 {
			filtered, err := filterPorts(ports, *containerPorts)
			if err != nil {
				return err
			}

			ports = filtered
		}

		if *name != "" && len(ports) > 1 {
			return fnerrors.New("--name can only be used when exposing a single port")
		}

		if len(ports) == 0 && *output == "plain" {
			fmt.Fprintf(console.Stdout(ctx), "Found no ports to export.\n")

			podCount, nscContainerCount, err := countApps(ctx, cluster)
			if err != nil {
				fmt.Fprintf(console.Debug(ctx), "Failed to create apps summary: %v\n", err)

				// Do not propagate errors to the user as this call is best-effort.
				return nil
			}

			if podCount > 0 && nscContainerCount == 0 {
				fmt.Fprintf(console.Stdout(ctx), "To expose apps from Kubernetes, run `nsc expose kubernetes` instead.\n")
			}
		}

		portNumbers := make([]int32, len(ports))
		for k, x := range ports {
			portNumbers[k] = x.ContainerPort
		}

		filledIn, err := fillInIngressRules(portNumbers, *ingressRules)
		if err != nil {
			return err
		}

		if len(ports) > 1 && *wildcard {
			return fnerrors.New("--wildcard is only supported when exposing a single port. Found %d", len(ports))
		}

		var exps []exported
		for k, port := range ports {
			resp, err := api.RegisterIngress(ctx, api.Methods, api.RegisterIngressRequest{
				ClusterId: cluster.ClusterId,
				Name:      *name,
				BackendEndpoint: &api.IngressBackendEndpoint{
					Port: port.ExportedPort,
				},
				HttpMatchRule: filledIn[k].HttpIngressRules,
				Wildcard:      *wildcard,
			}, cluster.ApiEndpoint)
			if err != nil {
				return err
			}

			exps = append(exps, exported{
				ContainerID:   port.ContainerID,
				ContainerName: port.ContainerName,
				ContainerPort: port.ContainerPort,
				URL:           "https://" + resp.Fqdn,
			})

			if *output == "plain" {
				fmt.Fprintf(console.Stdout(ctx), "Exported port %d from %s (%s):\n  https://%s\n\n",
					port.ContainerPort, port.ContainerName, substr(port.ContainerID), resp.Fqdn)
			}
		}

		if *output == "json" {
			slices.SortFunc(exps, func(a, b exported) int {
				return int(a.ContainerPort - b.ContainerPort)
			})

			return json.NewEncoder(console.Stdout(ctx)).Encode(exps)
		}

		return nil
	})

	return cmd
}

func filterPorts(ports []containerPort, acceptable []int) ([]containerPort, error) {
	var filtered []containerPort
	matched := map[int]struct{}{}
	for _, p := range ports {
		if slices.Contains(acceptable, int(p.ContainerPort)) {
			filtered = append(filtered, p)
			matched[int(p.ContainerPort)] = struct{}{}
		}
	}
	var unmatched []int
	for _, p := range acceptable {
		if _, ok := matched[p]; !ok {
			unmatched = append(unmatched, p)
		}
	}
	switch len(unmatched) {
	case 0:
		return filtered, nil

	case 1:
		return nil, fnerrors.New("specified port %d is not exported", unmatched[0])

	default:
		return nil, fnerrors.New("specified ports %s are not exported", strings.Join(stringify(unmatched), ", "))
	}
}

func stringify(values []int) []string {
	result := make([]string, len(values))
	for k, v := range values {
		result[k] = fmt.Sprintf("%d", v)
	}
	return result
}

type exported struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	ContainerPort int32  `json:"container_port"`
	URL           string `json:"url"`
}

type containerPort struct {
	ContainerID   string
	ContainerName string
	ContainerPort int32
	ExportedPort  int32
}

type portMap map[int]containerPort

type containerFilter struct {
	all           bool
	containerName string
}

func selectPorts(ctx context.Context, cluster *api.KubernetesCluster, source string, filter containerFilter) ([]containerPort, error) {
	return tasks.Return(ctx, tasks.Action("nsc.expose").HumanReadablef("Querying exported ports"), func(ctx context.Context) ([]containerPort, error) {
		if source != "" && source != "docker" && source != "containerd" {
			return nil, fnerrors.New("--source can be either empty, or one of %q or %q", "docker", "containerd")
		}

		eg := executor.New(ctx, "port selector")

		ports := make([][]containerPort, 2)

		if source == "" || source == "docker" {
			eg.Go(func(ctx context.Context) error {
				dockerPorts, err := selectDockerPorts(ctx, cluster, filter)
				if err != nil {
					return err
				}
				ports[0] = dockerPorts
				return nil
			})
		}

		if source == "" || source == "containerd" {
			eg.Go(func(ctx context.Context) error {
				containerdPorts, err := selectContainerdPorts(ctx, cluster, filter)
				if err != nil {
					return err
				}
				ports[1] = containerdPorts
				return nil
			})
		}

		if err := eg.Wait(); err != nil {
			return nil, err
		}

		return append(ports[0], ports[1]...), nil
	})
}

func selectDockerPorts(ctx context.Context, cluster *api.KubernetesCluster, filter containerFilter) ([]containerPort, error) {
	// We must fetch a token with our parent context, so we get a task sink etc.
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	docker, err := client.NewClientWithOpts(client.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return connectToDocker(ctx, token, cluster)
	}))
	if err != nil {
		return nil, err
	}

	defer docker.Close()

	data, err := dockerFilterToContainers(ctx, docker, filter)
	if err != nil {
		return nil, err
	}

	return buildContainersPortMap(ctx, data...)
}

func dockerFilterToContainers(ctx context.Context, docker *client.Client, filter containerFilter) ([]types.ContainerJSON, error) {
	if filter.all {
		list, err := docker.ContainerList(ctx, container.ListOptions{})
		if err != nil {
			return nil, err
		}

		actual := make([]types.ContainerJSON, len(list))
		for k, l := range list {
			res, err := docker.ContainerInspect(ctx, l.ID)
			if err != nil {
				return nil, err
			}
			actual[k] = res
		}

		return actual, nil
	}

	data, err := docker.ContainerInspect(ctx, filter.containerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil, nil
		}

		return nil, err
	}

	return []types.ContainerJSON{data}, nil
}

func buildContainersPortMap(ctx context.Context, data ...types.ContainerJSON) ([]containerPort, error) {
	exported := portMap{}
	for _, data := range data {
		internalName := parseContainerName(data.Name)

		for port, mapping := range data.NetworkSettings.Ports {
			if port.Proto() == "tcp" {
				for _, m := range mapping {
					if m.HostIP == "0.0.0.0" || m.HostIP == "::" {
						parsedPort, err := strconv.ParseInt(m.HostPort, 10, 32)
						if err != nil {
							return nil, err
						}

						exported[port.Int()] = containerPort{
							ContainerID:   data.ID,
							ContainerName: internalName,
							ContainerPort: int32(port.Int()),
							ExportedPort:  int32(parsedPort),
						}
					} else {
						fmt.Fprintf(console.Warnings(ctx), "%s: Skipping %d/%s exported to %s (unsupported)\n", data.Name, port.Int(), port.Proto(), m.HostIP)
					}
				}
			} else {
				fmt.Fprintf(console.Warnings(ctx), "%s: Skipping unsupported protocol %q, port %d\n", data.Name, port.Proto(), port.Int())
			}
		}
	}

	return maps.Values(exported), nil
}

func parseContainerName(name string) string {
	internalName := strings.TrimPrefix(name, "/") // docker returns names prefixed by /

	return internalName
}

func withContainerd(ctx context.Context, cluster *api.KubernetesCluster, callback func(context.Context, *containerd.Client) error) error {
	// We must fetch a token with our parent context, so we get a task sink etc.
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(fmt.Sprintf("passthrough:///%s-containerd", cluster.ClusterId),
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			vars := url.Values{}
			vars.Set("name", "containerd-socket")
			return api.DialHostedServiceWithToken(ctx, token, cluster, "unixsocket", vars)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}

	defer conn.Close()

	ctr, err := containerd.NewWithConn(conn)
	if err != nil {
		return err
	}

	defer ctr.Conn()

	ctx = namespaces.WithNamespace(ctx, "default")

	return callback(ctx, ctr)
}

func selectContainerdPorts(ctx context.Context, cluster *api.KubernetesCluster, filter containerFilter) ([]containerPort, error) {
	exported := portMap{}

	var filters []string

	if filter.containerName != "" {
		filters = append(filters,
			fmt.Sprintf("labels.%q==%s", labels.Name, filter.containerName),
			fmt.Sprintf("id~=^%s.*$", regexp.QuoteMeta(filter.containerName)),
		)
	}

	if err := withContainerd(ctx, cluster, func(ctx context.Context, ctr *containerd.Client) error {
		containers, err := ctr.Containers(ctx, filters...)
		if err != nil {
			return err
		}

		if len(containers) > 1 && filter.containerName != "" {
			return rpcerrors.Errorf(codes.InvalidArgument, "container name matches multiple containers")
		}

		for _, ctr := range containers {
			l, err := ctr.Labels(ctx)
			if err != nil {
				return err
			}

			ports, err := portutil.ParsePortsLabel(l)
			if err != nil {
				return err
			}

			internalName := parseContainerName(l[labels.Name])

			for _, p := range ports {
				if p.Protocol == "tcp" && (p.HostIP == "0.0.0.0" || p.HostIP == "::") {
					exported[int(p.ContainerPort)] = containerPort{
						ContainerID:   ctr.ID(),
						ContainerName: internalName,
						ContainerPort: p.ContainerPort,
						ExportedPort:  p.HostPort,
					}
				} else {
					fmt.Fprintf(console.Warnings(ctx), "Skipping %d/%s exported to %s (unsupported)\n", p.ContainerPort, p.Protocol, p.HostIP)
				}
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return maps.Values(exported), nil
}

func substr(id string) string {
	if len(id) > 8 {
		return id[8:]
	}

	return id
}

func countApps(ctx context.Context, cluster *api.KubernetesCluster) (int, int, error) {
	response, err := api.GetClusterSummary(ctx, api.Methods, cluster.ClusterId, []string{"pods", "nsc/containers"})
	if err != nil {
		return 0, 0, err
	}

	var podCount, nscContainerCount int
	for _, r := range response.Summary {
		switch r.Resource {
		case "nsc/containers":
			nscContainerCount += len(r.PerResource)

		case "pods":
			for _, r := range r.PerResource {
				if !slices.Contains(ctl.SystemNamespaces, r.Namespace) {
					podCount++
				}
			}
		}
	}

	return podCount, nscContainerCount, nil
}

func newExposeKubernetesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubernetes [instance-id]",
		Short: "Opens a public ingress to the specified Kubernentes service load balancer.",
		Args:  cobra.MaximumNArgs(1),
	}

	name := cmd.Flags().String("name", "", "If specified, set the name of the exposed ingress. Only permitted when exposing a single port. By default, ingress names are generated.")
	namespace := cmd.Flags().String("namespace", "", "Namespace of the service load balancer to expose.")
	service := cmd.Flags().String("service", "", "Name of the service load balancer to expose.")
	port := cmd.Flags().Int32("port", 0, "Which exported Load Balancer port to expose.")
	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	ingressRules := cmd.Flags().StringSlice("ingress", []string{}, "Specify ingress rules. Separate each rule with `,`.")
	wait := cmd.Flags().Bool("wait", false, "Wait until the provided service got has a valid ingress to expose.")
	wildcard := cmd.Flags().Bool("wildcard", false, "If set, generate a wildcard ingress for the exposed service.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, _, err := SelectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		if *namespace == "" {
			return fnerrors.New("--namespace is required")
		}

		if *service == "" {
			return fnerrors.New("--service is required")
		}

		backend, err := selectBackend(ctx, cluster, *namespace, *service, *port, *wait)
		if err != nil {
			return err
		}

		var rules []*api.ContainerPort_HttpMatchRule
		for _, rule := range *ingressRules {
			parsed, err := parseRule(rule)
			if err != nil {
				return err
			}
			rules = append(rules, parsed)
		}

		resp, err := api.RegisterIngress(ctx, api.Methods, api.RegisterIngressRequest{
			ClusterId:       cluster.ClusterId,
			Name:            *name,
			BackendEndpoint: backend,
			HttpMatchRule:   rules,
			Wildcard:        *wildcard,
		}, cluster.ApiEndpoint)
		if err != nil {
			return err
		}

		switch *output {
		case "json":
			var exported = struct {
				Name string `json:"string"`
				Port int32  `json:"port"`
				URL  string `json:"url"`
			}{
				Name: *name,
				Port: backend.Port,
				URL:  "https://" + resp.Fqdn,
			}

			if err := json.NewEncoder(console.Stdout(ctx)).Encode(exported); err != nil {
				return fnerrors.InternalError("failed to encode ingress output as JSON output: %w", err)
			}

		default:
			if *output != "" && *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", *output)
			}

			fmt.Fprintf(console.Stdout(ctx), "Exported port %d from %s/%s:\n  https://%s\n\n",
				backend.Port, *namespace, *service, resp.Fqdn)
		}

		return nil
	})

	return cmd
}

func selectBackend(ctx context.Context, cluster *api.KubernetesCluster, ns, service string, port int32, wait bool) (*api.IngressBackendEndpoint, error) {
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
			return nil, fnerrors.New("service %q is not of type %s (found type %s)", service, corev1.ServiceTypeLoadBalancer, svc.Spec.Type)
		}

		port, err := selectPort(svc, port)
		if err != nil {
			return nil, err
		}

		if wait {
			w, err := cli.CoreV1().Services(ns).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				return nil, err
			}
			defer w.Stop()

			for ev := range w.ResultChan() {
				fmt.Fprintf(console.Debug(ctx), "saw a new event\n")

				svc, ok := ev.Object.(*corev1.Service)
				if !ok {
					continue
				}

				if svc.Name != service {
					continue
				}

				ipAddr, err := selectIpAddr(svc)
				if err != nil {
					var noIp noIpError
					if errors.As(err, &noIp) {
						continue
					}

					return nil, err
				}

				return &api.IngressBackendEndpoint{
					IpAddr: ipAddr,
					Port:   port,
				}, nil
			}
		}

		ipAddr, err := selectIpAddr(svc)
		if err != nil {
			return nil, err
		}

		return &api.IngressBackendEndpoint{
			IpAddr: ipAddr,
			Port:   port,
		}, nil
	})
}

func selectPort(svc *corev1.Service, requestedPort int32) (int32, error) {
	var ports []int32
	for _, port := range svc.Spec.Ports {
		ports = append(ports, port.Port)
	}

	if requestedPort != 0 {
		if !slices.Contains(ports, requestedPort) {
			return 0, fnerrors.New("service %q does not export port %d. Found ports: %v.", svc.Name, requestedPort, ports)
		}

		return requestedPort, nil
	}

	switch len(ports) {
	case 0:
		return 0, fnerrors.New("service %q exposes no ports", svc.Name)
	case 1:
		return ports[0], nil
	default:
		return 0, fnerrors.New("Service %q exports multiple ports %v. Please select one with --port.", svc.Name, ports)
	}
}

type noIpError struct {
	service string
}

func (e noIpError) Error() string {
	return fmt.Sprintf("Service %q has no exported ip addresses. This is unexpected. Please contact support@namespace.so.", e.service)
}

func selectIpAddr(svc *corev1.Service) (string, error) {
	var ipAddrs []string
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if ingress.IP == "" {
			continue
		}

		ipAddrs = append(ipAddrs, ingress.IP)
	}

	switch len(ipAddrs) {
	case 0:
		return "", noIpError{svc.Name}
	case 1:
		return ipAddrs[0], nil
	default:
		return "", fnerrors.New("Service %q has multiple exported ip addresses %v. This is unexpected. Please contact support@namespace.so.", svc.Name, ipAddrs)
	}
}
