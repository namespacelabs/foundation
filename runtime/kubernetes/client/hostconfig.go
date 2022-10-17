// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import "namespacelabs.dev/foundation/std/cfg"

func NewLocalHostEnv(contextName string, env cfg.Context) *HostEnv {
	hostEnv := &HostEnv{
		Kubeconfig: "~/.kube/config",
		Context:    contextName,
	}

	return hostEnv
}
