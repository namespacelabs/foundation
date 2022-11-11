// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/std/tasks"
)

func computeSystemInfo(ctx context.Context, cli *kubernetes.Clientset) (*kubedef.SystemInfo, error) {
	var platforms uniquestrings.List

	nodes, err := cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fnerrors.InvocationError("kubernetes", "unable to list nodes: %w", err)
	}

	sysInfo := &kubedef.SystemInfo{}

	regions := map[string]int32{}
	zones := map[string]int32{}
	for _, n := range nodes.Items {
		platforms.Add(fmt.Sprintf("%s/%s", n.Status.NodeInfo.OperatingSystem, n.Status.NodeInfo.Architecture))

		if nodeType, ok := n.Labels["node.kubernetes.io/instance-type"]; ok && nodeType == "k3s" {
			sysInfo.DetectedDistribution = "k3s"
		}

		if clusterName, ok := n.Labels["alpha.eksctl.io/cluster-name"]; ok {
			sysInfo.DetectedDistribution = "eks"
			sysInfo.EksClusterName = clusterName
		}

		if region, ok := n.Labels["topology.kubernetes.io/region"]; ok {
			regions[region]++
		}

		if zone, ok := n.Labels["topology.kubernetes.io/zone"]; ok {
			zones[zone]++
		}
	}

	for region, count := range regions {
		sysInfo.RegionDistribution = append(sysInfo.RegionDistribution, &kubedef.NodeDistribution{
			Location: region,
			Count:    count,
		})
		sysInfo.Regions = append(sysInfo.Regions, region)
	}

	for zone, count := range zones {
		sysInfo.ZoneDistribution = append(sysInfo.ZoneDistribution, &kubedef.NodeDistribution{
			Location: zone,
			Count:    count,
		})
		sysInfo.Zones = append(sysInfo.Zones, zone)
	}

	sort.Strings(sysInfo.Regions)
	sort.Strings(sysInfo.Zones)

	less := func(a, b *kubedef.NodeDistribution) bool {
		return strings.Compare(a.Location, b.Location) < 0
	}

	slices.SortFunc(sysInfo.RegionDistribution, less)
	slices.SortFunc(sysInfo.ZoneDistribution, less)

	sysInfo.NodePlatform = platforms.Strings()
	sort.Strings(sysInfo.NodePlatform)
	tasks.Attachments(ctx).AddResult("platforms", sysInfo.NodePlatform)

	return sysInfo, nil
}
