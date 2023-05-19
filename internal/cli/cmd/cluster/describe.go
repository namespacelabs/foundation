// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

var kubernetesResources = []string{
	"pods",
	"namespaces",
	"services",
	"replicasets",
	"deployments",
	"statefulsets",
	"daemonsets",
}

var resources = append(kubernetesResources,
	"nsc/containers",
	"nsc/ingress",
)

func NewDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Outputs a description of the specified cluster.",
		Args:  cobra.ExactArgs(1),
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		cluster, args, err := selectRunningCluster(ctx, args)
		if err != nil {
			return err
		}

		if cluster == nil {
			return nil
		}

		response, err := api.GetClusterSummary(ctx, api.Endpoint, cluster.ClusterId, resources)
		if err != nil {
			return err
		}

		switch *output {
		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			return enc.Encode(response.Summary)

		default:
			if *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "defaulting output to plain\n")
			}

			count := 0
			for _, r := range response.Summary {
				if len(r.PerResource) > 0 && slices.Contains(kubernetesResources, r.Resource) {
					count++
				}
			}

			if count > 0 {
				fmt.Fprintf(console.Warnings(ctx), "The cluster containers Kubernetes resources which are not being output.\n")
			}

			var containers, ingresses []api.Resource

			for _, r := range response.Summary {
				if r.Resource == "nsc/containers" {
					containers = maps.Values(r.PerResource)
				} else if r.Resource == "nsc/ingress" {
					ingresses = maps.Values(r.PerResource)
				}
			}

			out := console.Stdout(ctx)
			fmt.Fprintf(out, "%s:\n", cluster.ClusterId)

			for k, ctr := range containers {
				if k > 0 {
					fmt.Fprintln(out)
				}
				fmt.Fprintf(out, "  Managed Container ID: %s\n", ctr.Uid)
				fmt.Fprintf(out, "  Name: %s\n", ctr.Name)
				fmt.Fprintf(out, "  Created: %s\n", ctr.CreationTime)

				if ctr.Tombstone != nil {
					fmt.Fprintf(out, "  Stopped: %s\n", ctr.Tombstone)
				}

				for _, x := range ctr.Container {
					fmt.Fprintf(out, "    Container %s\n", x.Id)
					for _, port := range x.ExportedPort {
						fmt.Fprintf(out, "      Exported port: %s/%d -> %d\n", port.Proto, port.ContainerPort, port.ExportedPort)
					}
				}

				for _, ingress := range ingresses {
					for _, x := range ctr.Container {
						if hasPort(x.ExportedPort, ingress.TargetExportedPort) {
							fmt.Fprintf(out, "  Ingress: https://%s%s\n", ingress.Name, ingressSuffix(ingress))
							break
						}
					}
				}
			}

			return nil
		}
	})

	return cmd
}

func ingressSuffix(ingress api.Resource) string {
	bits := map[string]struct{}{}

	for _, x := range ingress.HttpMatchRule {
		if x.DoesNotRequireAuth {
			bits["does not require auth"] = struct{}{}
		}
	}

	g := maps.Keys(bits)
	if len(g) > 0 {
		slices.Sort(g)
		return " (" + strings.Join(g, "; ") + ")"
	}

	return ""
}

func hasPort(ports []*api.Container_ExportedContainerPort, port int32) bool {
	for _, p := range ports {
		if p.ExportedPort == port {
			return true
		}
	}

	return false
}
