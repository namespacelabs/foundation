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

type K8sRuntime struct {
	Unbound
	env *fnschema.Environment
	ns  string
}

var _ runtime.Runtime = K8sRuntime{}

func resolveConfig(ctx context.Context, host *client.HostConfig) (*rest.Config, error) {
	config, err := client.NewRestConfigFromHostEnv(ctx, host)
	if err != nil {
		return nil, err
	}

	return client.CopyAndSetDefaults(*config, corev1.SchemeGroupVersion), nil
}
