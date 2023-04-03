// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newExposeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expose [cluster-id]",
		Short: "Opens a public ingress to the specified exported port.",
		Args:  cobra.MaximumNArgs(1),
	}

	source := cmd.Flags().String("source", "docker", "Where to lookup the container.")
	prefix := cmd.Flags().String("prefix", "", "If specified, prefixes the allocated URL.")
	containerName := cmd.Flags().String("container", "", "Which container to export.")
	containerPorts := cmd.Flags().IntSlice("container_port", nil, "If specified, only exposes the specified ports.")
	output := cmd.Flags().StringP("output", "o", "text", "One of text or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *containerName == "" {
			return fnerrors.New("--container is required")
		}

		cluster, args, err := selectCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		ports, err := selectPorts(ctx, cluster, *source, *containerName)
		if err != nil {
			return err
		}

		if len(*containerPorts) > 0 {
			remapped := portMap{}

			for _, port := range *containerPorts {
				if m, has := ports[port]; !has {
					return fnerrors.New("port %d not exported by container", port)
				} else {
					remapped[port] = m
				}
			}

			ports = remapped
		}

		var exps []exported
		for containerPort, port := range ports {
			p := *prefix
			if p == "" {
				p = port.SuggestedPrefix
			}

			resp, err := api.RegisterDefaultIngress(ctx, api.Endpoint, api.RegisterDefaultIngressRequest{
				ClusterId: cluster.ClusterId,
				Prefix:    p,
				BackendEndpoint: &api.IngressBackendEndpoint{
					Port: port.ExportedPort,
				},
			})
			if err != nil {
				return err
			}

			exps = append(exps, exported{
				ContainerID:   port.ContainerID,
				ContainerName: port.ContainerName,
				ContainerPort: int32(containerPort),
				URL:           "https://" + resp.Fqdn,
			})

			if *output == "text" {
				fmt.Fprintf(console.Stdout(ctx), "Exported port %d from %s (%s):\n  https://%s\n\n",
					containerPort, port.ContainerName, substr(port.ContainerID), resp.Fqdn)
			}
		}

		if *output == "json" {
			slices.SortFunc(exps, func(a, b exported) bool {
				return a.ContainerPort < b.ContainerPort
			})

			return json.NewEncoder(console.Stdout(ctx)).Encode(exps)
		}

		return nil
	})

	return cmd
}

type exported struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	ContainerPort int32  `json:"container_port"`
	URL           string `json:"url"`
}

type containerPort struct {
	ContainerID     string
	ContainerName   string
	SuggestedPrefix string
	ExportedPort    int32
}

type portMap map[int]containerPort

func selectPorts(ctx context.Context, cluster *api.KubernetesCluster, source, containerName string) (portMap, error) {
	switch source {
	case "docker":
		return selectDockerPorts(ctx, cluster, containerName)
	}

	return nil, fnerrors.New("unsupported source %q", source)
}

func selectDockerPorts(ctx context.Context, cluster *api.KubernetesCluster, containerName string) (portMap, error) {
	// We must fetch a token with our parent context, so we get a task sink etc.
	token, err := fnapi.FetchTenantToken(ctx)
	if err != nil {
		return nil, err
	}

	docker, err := client.NewClientWithOpts(client.WithDialContext(func(ctx context.Context, network, addr string) (net.Conn, error) {
		return api.DialPortWithToken(ctx, token, cluster, 2375)
	}))
	if err != nil {
		return nil, err
	}

	defer docker.Close()

	data, err := docker.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, err
	}

	internalName := strings.TrimPrefix(data.Name, "/")        // docker returns names prefixed by /
	mangledName := strings.ReplaceAll(internalName, "_", "-") // docker generated names have underscores.

	suggestedPrefix := computeSuggestedPrefix(data.ID, mangledName)

	exported := portMap{}
	for port, mapping := range data.NetworkSettings.Ports {
		if port.Proto() == "tcp" && len(mapping) > 0 {
			for _, m := range mapping {
				if m.HostIP == "0.0.0.0" {
					parsedPort, err := strconv.ParseInt(m.HostPort, 10, 32)
					if err != nil {
						return nil, err
					}

					exported[port.Int()] = containerPort{
						ContainerID:     data.ID,
						ContainerName:   internalName,
						SuggestedPrefix: suggestedPrefix,
						ExportedPort:    int32(parsedPort),
					}
				}
			}
		}
	}

	return exported, nil
}

var simpleLabelRe = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-]*$")

func computeSuggestedPrefix(id, name string) string {
	if len(name) < 24 && simpleLabelRe.MatchString(name) {
		return name
	}

	return substr(id)
}

func substr(id string) string {
	if len(id) > 8 {
		return id[8:]
	}

	return id
}
