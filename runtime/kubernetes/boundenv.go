// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
)

type K8sRuntime struct {
	Unbound
	moduleNamespace string
}

var _ runtime.Runtime = K8sRuntime{}

func resolveConfig(ctx context.Context, host *client.HostConfig) (*rest.Config, error) {
	config, err := client.NewRestConfigFromHostEnv(ctx, host)
	if err != nil {
		return nil, err
	}

	// Obtained from kubectl_match_version.go.
	config.GroupVersion = &schema.GroupVersion{Group: "", Version: "v1"}

	if config.APIPath == "" {
		config.APIPath = "/api"
	}

	if config.NegotiatedSerializer == nil {
		// This codec factory ensures the resources are not converted. Therefore, resources
		// will not be round-tripped through internal versions. Defaulting does not happen
		// on the client.
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	}

	if err := rest.SetKubernetesDefaults(config); err != nil {
		return nil, err
	}

	return config, err
}
