// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"k8s.io/utils/strings/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func SelectService(rt *runtimepb.RuntimeConfig, ref *schema.ServiceRef) (*runtimepb.Server_Service, error) {
	if ref == nil {
		return nil, fnerrors.BadInputError("missing required service endpoint")
	}

	allServers := append(rt.StackEntry, rt.Current)

	for _, srv := range allServers {
		if srv.PackageName == ref.GetServerRef().GetPackageName() {
			for _, service := range srv.Service {
				if service.Name == ref.ServiceName {
					return service, nil
				}
			}

			return nil, fnerrors.BadInputError("the required service %q is not exported by %q",
				ref.ServiceName, ref.GetServerRef().GetPackageName())
		}
	}

	return nil, fnerrors.BadInputError("the required server %q is not present in the stack", ref.GetServerRef().GetPackageName())
}

func SelectServiceValue(rt *runtimepb.RuntimeConfig, ref *schema.ServiceRef, selector func(*runtimepb.Server_Service) (string, error)) (string, error) {
	svc, err := SelectService(rt, ref)
	if err != nil {
		return "", err
	}

	return selector(svc)
}

func SelectServiceEndpoint(svc *runtimepb.Server_Service) (string, error) {
	return svc.Endpoint, nil
}

func SelectServiceIngress(service *runtimepb.Server_Service) (string, error) {
	if service.Ingress == nil || len(service.Ingress.Domain) == 0 {
		return "", fnerrors.BadInputError("service %s has no ingress, %v", service.Name, service)
	}

	// TODO: introduce a concept of the "default" ingress, use it here.
	return service.Ingress.Domain[0].BaseUrl, nil
}

func SelectInstance(rt *runtimepb.RuntimeConfig, instance *schema.FieldSelector_Instance) (any, error) {
	switch {
	case instance.Service != nil:
		return SelectService(rt, instance.Service)

	case instance.SelectInternalEndpointByKind != "":
		var matches []*runtimepb.Server_InternalEndpoint
		for _, m := range rt.Current.GetInternalEndpoint() {
			if slices.Contains(m.Kinds, instance.SelectInternalEndpointByKind) {
				matches = append(matches, m)
			}
		}
		if len(matches) != 1 {
			return nil, fnerrors.BadInputError("%s: expected 1 match, got %d", instance.SelectInternalEndpointByKind, len(matches))
		}
		return matches[0], nil
	}

	return nil, fnerrors.BadInputError("instance: can't construct a value")
}
