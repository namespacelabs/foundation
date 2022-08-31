// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
)

func serverToRuntimeConfig(server provision.Server) (*types.ServerRuntimeConfig, error) {
	config := &types.ServerRuntimeConfig{}
	for _, service := range server.Proto().Service {
		config.Services = append(config.Services, convertService(service))
	}
	for _, service := range server.Proto().Ingress {
		config.Services = append(config.Services, convertService(service))
	}
	return config, nil
}

func convertService(service *schema.Server_ServiceSpec) *types.ServiceRuntimeConfig {
	return &types.ServiceRuntimeConfig{
		Name: service.Name,
		Port: service.Port.ContainerPort,
	}
}
