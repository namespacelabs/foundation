// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type systemInfo struct {
	nodePlatforms []string
}

type fetchSystemInfo struct {
	cli *k8s.Clientset
	cfg *client.HostEnv

	compute.DoScoped[systemInfo] // DoScoped so the computation is deferred.
}

var _ compute.Computable[systemInfo] = &fetchSystemInfo{}

func (f *fetchSystemInfo) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.fetch-system-info")
}

func (f *fetchSystemInfo) Inputs() *compute.In {
	return compute.Inputs().Proto("cfg", f.cfg)
}

func (f *fetchSystemInfo) Output() compute.Output { return compute.Output{NotCacheable: true} }

func (f *fetchSystemInfo) Compute(ctx context.Context, _ compute.Resolved) (systemInfo, error) {
	var platforms uniquestrings.List

	nodes, err := f.cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return systemInfo{}, err
	}

	for _, n := range nodes.Items {
		platforms.Add(fmt.Sprintf("%s/%s", n.Status.NodeInfo.OperatingSystem, n.Status.NodeInfo.Architecture))
	}

	nodePlatforms := platforms.Strings()
	sort.Strings(nodePlatforms)
	tasks.Attachments(ctx).AddResult("platforms", nodePlatforms)

	return systemInfo{nodePlatforms: nodePlatforms}, nil
}
