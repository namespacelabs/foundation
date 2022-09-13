// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	fnschema "namespacelabs.dev/foundation/schema"
)

type ClusterNamespace struct {
	cluster *Cluster
	target  clusterTarget
}

type clusterTarget struct {
	env       *fnschema.Environment
	namespace string
}

var _ runtime.ClusterNamespace = &ClusterNamespace{}

func (cn *ClusterNamespace) Cluster() runtime.Cluster {
	return cn.cluster
}

func (cn *ClusterNamespace) Planner() runtime.Planner {
	return Planner{fetchSystemInfo: cn.cluster.SystemInfo, target: cn.target}
}

func (u *ClusterNamespace) HostConfig() *client.HostConfig {
	return u.cluster.HostConfig()
}

func resolveConfig(ctx context.Context, host *client.HostConfig) (*rest.Config, error) {
	config, err := client.NewRestConfigFromHostEnv(ctx, host)
	if err != nil {
		return nil, err
	}

	return client.CopyAndSetDefaults(*config, corev1.SchemeGroupVersion), nil
}
