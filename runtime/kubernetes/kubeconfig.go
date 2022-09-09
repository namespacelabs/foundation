// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

type KubeConfig struct {
	Config, Context, Namespace string
}

func (r ClusterNamespace) KubeConfig() KubeConfig {
	return KubeConfig{
		Config:    r.host.HostEnv.Kubeconfig,
		Context:   r.host.HostEnv.Context,
		Namespace: r.namespace,
	}
}
