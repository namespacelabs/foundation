// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"fmt"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/runtime"
)

func serverToRuntimeConfig(stack *planning.Stack, ps planning.PlannedServer, serverImage oci.ImageID) (*runtime.RuntimeConfig, error) {
	srv := ps.Server
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(srv.SealedContext().Environment()),
		Current:     makeServerConfig(stack, srv),
	}

	config.Current.ImageRef = serverImage.String()

	for _, pkg := range ps.DeclaredStack.PackageNames() {
		if pkg == ps.PackageName() {
			continue
		}

		ref, ok := stack.Get(pkg)
		if !ok {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}

		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref.Server))
	}

	return config, nil
}

func TestStackToRuntimeConfig(stack *planning.Stack, sutServers []string) (*runtime.RuntimeConfig, error) {
	if len(sutServers) == 0 {
		return nil, fnerrors.InternalError("no servers to test")
	}

	config := &runtime.RuntimeConfig{
		Environment: makeEnv(stack.Servers[0].Server.SealedContext().Environment()),
	}

	for _, pkg := range sutServers {
		ref, ok := stack.Get(schema.MakePackageName(pkg))
		if !ok {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}

		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref.Server))
	}

	return config, nil
}

func makeEnv(env *schema.Environment) *runtime.RuntimeConfig_Environment {
	res := &runtime.RuntimeConfig_Environment{
		Ephemeral: env.Ephemeral,
		Purpose:   env.Purpose.String(),
	}

	// Ephemeral environments use generated names, that should not be depended on.
	if !env.Ephemeral {
		res.Name = env.Name
	}

	return res
}

func makeServerConfig(stack *planning.Stack, server planning.Server) *runtime.Server {
	current := &runtime.Server{
		PackageName: server.Proto().PackageName,
		ModuleName:  server.Proto().ModuleName,
	}

	for _, service := range server.Proto().Service {
		current.Port = append(current.Port, makePort(service))
	}

	for _, service := range server.Proto().Ingress {
		current.Port = append(current.Port, makePort(service))
	}

	for _, endpoint := range stack.Endpoints {
		if endpoint.ServerOwner != server.Proto().PackageName {
			continue
		}

		current.Service = append(current.Service, &runtime.Server_Service{
			Owner:    endpoint.EndpointOwner,
			Name:     endpoint.ServiceName,
			Endpoint: fmt.Sprintf("%s:%d", endpoint.AllocatedName, endpoint.Port.ContainerPort),
		})
	}

	return current
}

func makePort(service *schema.Server_ServiceSpec) *runtime.Server_Port {
	return &runtime.Server_Port{
		Name: service.Name,
		Port: service.Port.ContainerPort,
	}
}
