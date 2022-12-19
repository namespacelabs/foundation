// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"fmt"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	internalruntime "namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/runtime"
)

func serverToRuntimeConfig(stack serverStack, ps planning.PlannedServer, serverImage oci.ImageID, ingressFragments []*schema.IngressFragment) (*runtime.RuntimeConfig, error) {
	srv := ps.Server
	env := srv.SealedContext().Environment()
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(env),
		Current:     makeServerConfig(stack, srv.Proto(), env, ingressFragments),
	}

	config.Current.ImageRef = serverImage.String()

	for _, pkg := range ps.DeclaredStack.PackageNames() {
		if pkg == ps.PackageName() {
			continue
		}

		ref, ok := stack.GetServerProto(pkg)
		if !ok {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}

		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref, env, ingressFragments))
	}

	return config, nil
}

func TestStackToRuntimeConfig(stack *planning.Stack, sutServers []string, ingressFragments []*schema.IngressFragment) (*runtime.RuntimeConfig, error) {
	if len(sutServers) == 0 {
		return nil, fnerrors.InternalError("no servers to test")
	}

	env := stack.Servers[0].Server.SealedContext().Environment()
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(env),
	}

	for _, pkg := range sutServers {
		ref, ok := stack.GetServerProto(schema.MakePackageName(pkg))
		if !ok {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}

		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref, env, ingressFragments))
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

func makeServerConfig(stack serverStack, server *schema.Server, env *schema.Environment, ingressFragments []*schema.IngressFragment) *runtime.Server {
	current := &runtime.Server{
		PackageName: server.PackageName,
		ModuleName:  server.ModuleName,
	}

	for _, service := range server.Service {
		current.Port = append(current.Port, makePort(service))
	}

	for _, service := range server.Ingress {
		current.Port = append(current.Port, makePort(service))
	}

	endpoints, _ := stack.GetEndpoints(schema.PackageName(server.PackageName))
	for _, endpoint := range endpoints {
		current.Service = append(current.Service, &runtime.Server_Service{
			Owner:    endpoint.EndpointOwner,
			Name:     endpoint.ServiceName,
			Endpoint: fmt.Sprintf("%s:%d", endpoint.AllocatedName, endpoint.Port.ContainerPort),
			Ingress:  makeServiceIngress(endpoint, env, ingressFragments),
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

// TODO: consolidate with "resolveBackend" from Node.js build
func makeServiceIngress(endpoint *schema.Endpoint, env *schema.Environment, ingressFragments []*schema.IngressFragment) *runtime.Server_Ingress {
	var matching []*schema.IngressFragment

	for _, fragment := range ingressFragments {
		if fragment.GetOwner() != endpoint.EndpointOwner {
			continue
		}

		if fragment.GetEndpoint().GetServiceName() != endpoint.ServiceName {
			continue
		}

		matching = append(matching, fragment)
	}

	// There is often no ingress in tests so we use in-cluster addresses.
	// In the future we could allow the user to annotate domains which would
	// be accessible from the test environment.
	useInClusterAddresses := env.Purpose == schema.Environment_TESTING

	ingress := &runtime.Server_Ingress{}
	for _, fragment := range matching {
		domain := &runtime.Server_Ingress_Domain{}

		if useInClusterAddresses {
			domain.BaseUrl = fmt.Sprintf("http://%s:%d", fragment.Endpoint.AllocatedName, fragment.Endpoint.Port.ContainerPort)
		} else {
			d := fragment.Domain
			if d.Managed == schema.Domain_LOCAL_MANAGED {
				domain.BaseUrl = fmt.Sprintf("http://%s:%d", d.Fqdn, internalruntime.LocalIngressPort)
			} else {
				if d.TlsFrontend {
					domain.BaseUrl = fmt.Sprintf("https://%s", d.Fqdn)
				} else {
					domain.BaseUrl = fmt.Sprintf("http://%s", d.Fqdn)
				}
			}
		}

		ingress.Domain = append(ingress.Domain, domain)
	}
	return ingress
}
