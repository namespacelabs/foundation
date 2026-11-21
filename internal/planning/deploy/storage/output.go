// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storage

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

type PortFwd struct {
	Endpoint  *fnschema.Endpoint
	LocalPort uint32
}

func ToStorageNetworkPlan(localHostname string, stack *fnschema.Stack, focus []fnschema.PackageName, portFwds []*PortFwd, ingressFragments []*fnschema.IngressFragment) *storage.NetworkPlan {
	r := &storage.NetworkPlan{
		LocalHostname: localHostname,
	}

	r.FocusedServerPackages = fnschema.Strs(focus...)

	for _, i := range ingressFragments {
		domain := convertDomain(i.Domain)
		endpoint := convertEndpoint(i.Endpoint, 0 /* localPort */, stack)
		fragment := &storage.IngressFragment{
			Name:     i.Name,
			Owner:    i.Owner,
			Domain:   domain,
			Endpoint: endpoint,
			Manager:  i.Manager,
		}
		for _, httpPath := range i.HttpPath {
			fragment.HttpPath = append(fragment.HttpPath, convertHttpPath(httpPath))
		}
		for _, grpcService := range i.GrpcService {
			fragment.GrpcService = append(fragment.GrpcService, convertGrpcService(grpcService))
		}
		r.IngressFragments = append(r.IngressFragments, fragment)
	}

	for _, pfwd := range portFwds {
		endpoint := convertEndpoint(pfwd.Endpoint, pfwd.LocalPort, stack)
		r.Endpoints = append(r.Endpoints, endpoint)
	}

	return r
}

func convertDomain(d *fnschema.Domain) *storage.Domain {
	if d == nil {
		return nil
	}

	managedType, _ := convertManagedType(d.Managed)
	return &storage.Domain{
		Fqdn:        d.Fqdn,
		Managed:     managedType,
		TlsFrontend: d.TlsFrontend,
	}
}

func convertGrpcService(s *fnschema.IngressFragment_IngressGrpcService) *storage.IngressGrpcService {
	return &storage.IngressGrpcService{
		GrpcService: s.GrpcService,
		Owner:       s.Owner,
		Service:     s.Service,
		Method:      s.Method,
		ServicePort: s.ServicePort,
		BackendTls:  s.BackendTls,
		AllServices: s.AllServices,
	}
}

func convertEndpoint(endpoint *fnschema.Endpoint, localPort uint32, stack *fnschema.Stack) *storage.Endpoint {
	if endpoint == nil {
		return nil
	}

	entry := stack.GetServer(fnschema.PackageName(endpoint.EndpointOwner))
	serverName := ""
	if entry != nil {
		serverName = entry.Server.Name
	}

	endpointType, _ := convertEndpointType(endpoint.Type)
	result := &storage.Endpoint{
		Type:          endpointType,
		ServiceName:   endpoint.ServiceName,
		EndpointOwner: endpoint.EndpointOwner,
		Ports:         endpoint.Ports,
		AllocatedName: endpoint.AllocatedName,
		ServerOwner:   endpoint.ServerOwner,
		ServiceLabel:  endpoint.ServiceLabel,
		LocalPort:     localPort,
		ServerName:    serverName,
	}
	for _, m := range endpoint.ServiceMetadata {
		result.ServiceMetadata = append(result.ServiceMetadata, &storage.Endpoint_ServiceMetadata{
			Kind:     m.Kind,
			Protocol: m.Protocol,
		})
	}

	return result
}

func convertEndpointType(t fnschema.Endpoint_Type) (storage.Endpoint_Type, error) {
	switch t {
	case fnschema.Endpoint_INGRESS_UNSPECIFIED:
		return storage.Endpoint_INGRESS_UNSPECIFIED, nil
	case fnschema.Endpoint_PRIVATE:
		return storage.Endpoint_PRIVATE, nil
	case fnschema.Endpoint_INTERNET_FACING:
		return storage.Endpoint_INTERNET_FACING, nil
	case fnschema.Endpoint_LOAD_BALANCER:
		return storage.Endpoint_LOAD_BALANCER, nil
	default:
		return storage.Endpoint_INGRESS_UNSPECIFIED, fnerrors.InternalError("unknown endpoint type: %s", t)
	}
}

func convertHttpPath(httpPath *fnschema.IngressFragment_IngressHttpPath) *storage.IngressHttpPath {
	return &storage.IngressHttpPath{
		Path:        httpPath.Path,
		Kind:        httpPath.Kind,
		Owner:       httpPath.Owner,
		Service:     httpPath.Service,
		ServicePort: httpPath.ServicePort,
	}
}

func convertManagedType(managed fnschema.Domain_ManagedType) (storage.Domain_ManagedType, error) {
	switch managed {
	case fnschema.Domain_MANAGED_UNKNOWN:
		return storage.Domain_MANAGED_UNKNOWN, nil
	case fnschema.Domain_LOCAL_MANAGED:
		return storage.Domain_LOCAL_MANAGED, nil
	case fnschema.Domain_CLOUD_MANAGED:
		return storage.Domain_CLOUD_MANAGED, nil
	case fnschema.Domain_CLOUD_TERMINATION:
		return storage.Domain_CLOUD_TERMINATION, nil
	case fnschema.Domain_USER_SPECIFIED:
		return storage.Domain_USER_SPECIFIED, nil
	case fnschema.Domain_USER_SPECIFIED_TLS_MANAGED:
		return storage.Domain_USER_SPECIFIED_TLS_MANAGED, nil
	default:
		return storage.Domain_MANAGED_UNKNOWN, fnerrors.InternalError("unknown domain managed type: %s", managed)
	}
}
