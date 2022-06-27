// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type fetchSystemInfo struct {
	cli *k8s.Clientset
	cfg *client.HostEnv

	compute.DoScoped[*kubedef.SystemInfo] // DoScoped so the computation is deferred.
}

var _ compute.Computable[*kubedef.SystemInfo] = &fetchSystemInfo{}

func (f *fetchSystemInfo) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.fetch-system-info")
}

func (f *fetchSystemInfo) Inputs() *compute.In {
	return compute.Inputs().Proto("cfg", f.cfg)
}

func (f *fetchSystemInfo) Output() compute.Output { return compute.Output{NotCacheable: true} }

func (f *fetchSystemInfo) Compute(ctx context.Context, _ compute.Resolved) (*kubedef.SystemInfo, error) {
	var platforms uniquestrings.List

	nodes, err := f.cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fnerrors.Wrapf(nil, err, "unable to list nodes")
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
