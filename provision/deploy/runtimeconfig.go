// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"fmt"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/runtime"
)

var privateEntries schema.PackageList

func init() {
	privateEntries.Add("namespacelabs.dev/foundation/std/runtime/kubernetes/controller") // Don't include the kube controller as a dep.
}

func serverToRuntimeConfig(stack *stack.Stack, server provision.Server, serverImage oci.ImageID) (*runtime.RuntimeConfig, error) {
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(server.SealedContext().Environment()),
		Current:     makeServer(stack, server),
	}

	config.Current.ImageRef = serverImage.String()

	for _, pkg := range stack.GetParsed(server.PackageName()).DeclaredStack.PackageNames() {
		if pkg == server.PackageName() || privateEntries.Includes(pkg) {
			continue
		}

		ref := stack.Get(pkg)
		if ref == nil {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}

		config.StackEntry = append(config.StackEntry, makeServer(stack, *ref))
	}

	return config, nil
}

func makeEnv(env *schema.Environment) *runtime.ServerEnvironment {
	res := &runtime.ServerEnvironment{
		Ephemeral: env.Ephemeral,
		Purpose:   env.Purpose.String(),
	}

	// Ephemeral environments use generated names, that should not be depended on.
	if !env.Ephemeral {
		res.Name = env.Name
	}

	return res
}

func makeServer(stack *stack.Stack, server provision.Server) *runtime.Server {
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

		current.Service = append(current.Service, &runtime.Service{
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
