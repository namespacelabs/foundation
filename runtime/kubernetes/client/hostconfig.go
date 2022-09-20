// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func NewLocalHostEnv(contextName string, env planning.Context) (*HostEnv, error) {
	kubeconfig, err := dirs.ExpandHome("~/.kube/config")
	if err != nil {
		return nil, err
	}

	hostEnv := &HostEnv{
		Kubeconfig: kubeconfig,
		Context:    contextName,
	}

	return hostEnv, nil
}
