// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"strings"

	"namespacelabs.dev/foundation/schema"
)

const (
	K8sServerId           = "k8s.namespacelabs.dev/server-id"
	K8sServerPackageName  = "k8s.namespacelabs.dev/server-package-name"
	K8sServicePackageName = "k8s.namespacelabs.dev/service-package-name"
	K8sServiceGrpcType    = "k8s.namespacelabs.dev/service-grpc-type"
	K8sEnvName            = "k8s.namespacelabs.dev/env"
	K8sEnvEphemeral       = "k8s.namespacelabs.dev/env-ephemeral"
	K8sEnvPurpose         = "k8s.namespacelabs.dev/env-purpose"
	K8sConfigImage        = "k8s.namespacelabs.dev/config-image"

	AppKubernetesIoManagedBy = "app.kubernetes.io/managed-by"

	id              = "foundation.namespace.so" // #220 Update when product name is final
	K8sFieldManager = id
)

func SelectById(srv *schema.Server) map[string]string {
	return map[string]string{
		K8sServerId: srv.Id,
	}
}

func SelectEphemeral() map[string]string {
	return map[string]string{
		K8sEnvEphemeral: "true",
	}
}

func ManagedBy() map[string]string {
	return map[string]string{
		AppKubernetesIoManagedBy: id,
	}
}

func MakeLabels(env *schema.Environment, srv *schema.Server) map[string]string {
	// XXX add recommended labels https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	m := ManagedBy()
	if srv != nil {
		m[K8sServerId] = srv.Id
	}
	if env != nil {
		m[K8sEnvName] = env.Name
		m[K8sEnvPurpose] = strings.ToLower(env.Purpose.String())
		if env.Ephemeral {
			m[K8sEnvEphemeral] = "true"
		} else {
			m[K8sEnvEphemeral] = "false"
		}
	}
	return m
}

func MakeAnnotations(entry *schema.Stack_Entry) map[string]string {
	m := map[string]string{
		K8sServerPackageName: entry.GetPackageName().String(),
	}

	// XXX add annotations with pointers to tools, team owners, etc.
	return m
}

func MakeServiceAnnotations(srv *schema.Server, endpoint *schema.Endpoint) (map[string]string, error) {
	m := map[string]string{
		K8sServicePackageName: endpoint.GetEndpointOwner(),
	}

	var grpcServices []string
	for _, p := range endpoint.ServiceMetadata {
		if p.Protocol == schema.GrpcProtocol {
			if p.Details == nil {
				continue
			}

			grpc := &schema.GrpcExportService{}
			if err := p.Details.UnmarshalTo(grpc); err != nil {
				return nil, err
			}

			grpcServices = append(grpcServices, grpc.ProtoTypename)
		}
	}

	if len(grpcServices) > 0 {
		m[K8sServiceGrpcType] = strings.Join(grpcServices, ",")
	}

	return m, nil
}

func MakeServiceLabels(env *schema.Environment, srv *schema.Server, endpoint *schema.Endpoint) map[string]string {
	m := MakeLabels(env, srv)

	return m
}
