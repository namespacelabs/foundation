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
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type fetchSystemInfo struct {
	cli     *k8s.Clientset
	cfg     *client.HostEnv
	devHost *schema.DevHost
	env     *schema.Environment

	compute.DoScoped[*client.SystemInfo] // DoScoped so the computation is deferred.
}

var _ compute.Computable[*client.SystemInfo] = &fetchSystemInfo{}

func (f *fetchSystemInfo) Action() *tasks.ActionEvent {
	return tasks.Action("kubernetes.fetch-system-info")
}

func (f *fetchSystemInfo) Inputs() *compute.In {
	return compute.Inputs().Proto("cfg", f.cfg).Proto("devhost", f.devHost).Proto("env", f.env)
}

func (f *fetchSystemInfo) Output() compute.Output { return compute.Output{NotCacheable: true} }

func (f *fetchSystemInfo) Compute(ctx context.Context, _ compute.Resolved) (*client.SystemInfo, error) {
	var platforms uniquestrings.List

	nodes, err := f.cli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	sysInfo := &client.SystemInfo{}

	for _, n := range nodes.Items {
		platforms.Add(fmt.Sprintf("%s/%s", n.Status.NodeInfo.OperatingSystem, n.Status.NodeInfo.Architecture))

		if nodeType, ok := n.Labels["node.kubernetes.io/instance-type"]; ok && nodeType == "k3s" {
			sysInfo.DetectedDistribution = "k3s"
		}

		if clusterName, ok := n.Labels["alpha.eksctl.io/cluster-name"]; ok {
			sysInfo.DetectedDistribution = "eks"
			sysInfo.EksClusterName = clusterName
		}
	}

	switch sysInfo.DetectedDistribution {
	case "eks":
		account, err := auth.Resolve(ctx, f.devHost, f.env)
		if err != nil {
			return nil, err
		}

		output, err := compute.GetValue(ctx, account)
		if err != nil {
			return nil, err
		}

		sysInfo.EksAccount = *output.Account
	}

	sysInfo.NodePlatform = platforms.Strings()
	sort.Strings(sysInfo.NodePlatform)
	tasks.Attachments(ctx).AddResult("platforms", sysInfo.NodePlatform)

	return sysInfo, nil
}
