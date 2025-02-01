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

func serverToRuntimeConfig(stack *planning.StackWithIngress, ps planning.PlannedServer, serverImage oci.ImageID) (*runtime.RuntimeConfig, error) {
	srv := ps.Server
	env := srv.SealedContext().Environment()
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(env),
		Current:     makeServerConfig(stack, ps, env),
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

		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref, env))
	}

	for _, cfg := range ps.MergedFragment.Listener {
		config.ListenerConfiguration = append(config.ListenerConfiguration, &runtime.RuntimeConfig_ListenerConfiguration{
			Name:          cfg.Name,
			Protocol:      cfg.Protocol,
			ContainerPort: cfg.GetPort().GetContainerPort(),
		})
	}

	return config, nil
}

func TestStackToRuntimeConfig(stack *planning.StackWithIngress, sutServers []string) (*runtime.RuntimeConfig, error) {
	if len(sutServers) == 0 {
		return nil, fnerrors.InternalError("no servers to test")
	}

	env := stack.Servers[0].Server.SealedContext().Environment()
	config := &runtime.RuntimeConfig{
		Environment: makeEnv(env),
	}

	for _, pkg := range sutServers {
		ref, ok := stack.Get(schema.MakePackageName(pkg))
		if !ok {
			return nil, fnerrors.InternalError("%s: missing in the stack", pkg)
		}
		config.StackEntry = append(config.StackEntry, makeServerConfig(stack, ref, env))
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

func makeServerConfig(stack *planning.StackWithIngress, srv planning.PlannedServer, env *schema.Environment) *runtime.Server {
	server := srv.MergedFragment

	current := &runtime.Server{
		ServerId:    srv.Proto().GetId(),
		PackageName: srv.PackageName().String(),
		ModuleName:  srv.Module().ModuleName(),
	}

	for _, service := range server.Service {
		for _, port := range service.Ports {
			current.Port = append(current.Port, makePort(service, port))
		}
	}

	for _, service := range server.Ingress {
		for _, port := range service.Ports {
			current.Port = append(current.Port, makePort(service, port))
		}
	}

	s, _ := stack.Get(srv.PackageName())
	for _, endpoint := range s.Endpoints {
		svc := &runtime.Server_Service{
			Owner:   endpoint.EndpointOwner,
			Name:    endpoint.ServiceName,
			Ingress: makeServiceIngress(stack, endpoint, env),
		}

		if len(endpoint.Ports) > 0 {
			svc.Endpoint = fmt.Sprintf("%s:%d", endpoint.AllocatedName, endpoint.Ports[0].ExportedPort)
			if endpoint.FullyQualifiedName != "" {
				svc.FullyQualifiedEndpoint = fmt.Sprintf("%s:%d", endpoint.FullyQualifiedName, endpoint.Ports[0].ExportedPort)
			}
		}

		current.Service = append(current.Service, svc)
	}

	for _, endpoint := range s.InternalEndpoints {
		ie := &runtime.Server_InternalEndpoint{
			PortName:      endpoint.Port.Name,
			ContainerPort: endpoint.Port.ContainerPort,
		}

		for _, md := range endpoint.ServiceMetadata {
			exported := &schema.HttpExportedService{}
			// XXX silent skipping.
			if md.Details.MessageIs(exported) && md.Details.UnmarshalTo(exported) == nil {
				ie.Exported = append(ie.Exported, &runtime.Server_InternalEndpoint_ExportedHttp{
					Path: exported.Path,
					Kind: md.Kind,
				})
			}
		}

		current.InternalEndpoint = append(current.InternalEndpoint, ie)
	}

	return current
}

func makePort(service *schema.Server_ServiceSpec, pm *schema.Endpoint_PortMap) *runtime.Server_Port {
	return &runtime.Server_Port{
		Name: service.Name,
		Port: pm.Port.ContainerPort,
	}
}

// TODO: consolidate with "resolveBackend" from Node.js build
func makeServiceIngress(stack *planning.StackWithIngress, endpoint *schema.Endpoint, env *schema.Environment) *runtime.Server_Ingress {
	// There is often no ingress in tests so we use the in-cluster address.
	// In the future we could allow the user to annotate domains which would
	// be accessible from the test environment.
	if env.Purpose == schema.Environment_TESTING {
		if len(endpoint.Ports) == 0 {
			return &runtime.Server_Ingress{}
		}

		return &runtime.Server_Ingress{
			Domain: []*runtime.Server_Ingress_Domain{{
				BaseUrl: fmt.Sprintf("http://%s:%d", endpoint.AllocatedName, endpoint.Ports[0].ExportedPort),
			}},
		}
	}

	ingress := &runtime.Server_Ingress{}
	for _, fragment := range stack.GetIngressesForService(endpoint.EndpointOwner, endpoint.ServiceName) {
		domain := &runtime.Server_Ingress_Domain{}

		d := fragment.Domain
		if d.Managed == schema.Domain_LOCAL_MANAGED {
			domain.BaseUrl = fmt.Sprintf("http://%s:%d", d.Fqdn, internalruntime.LocalIngressPort)
		} else if d.TlsFrontend {
			domain.BaseUrl = fmt.Sprintf("https://%s", d.Fqdn)
		} else {
			domain.BaseUrl = fmt.Sprintf("http://%s", d.Fqdn)
		}

		ingress.Domain = append(ingress.Domain, domain)
	}
	return ingress
}
