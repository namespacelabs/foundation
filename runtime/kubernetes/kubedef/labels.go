// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"strings"
	"time"

	"namespacelabs.dev/foundation/schema"
)

const (
	K8sServerId           = "k8s.namespacelabs.dev/server-id"
	K8sServerFocus        = "k8s.namespacelabs.dev/server-focus"
	K8sServerPackageName  = "k8s.namespacelabs.dev/server-package-name"
	K8sServicePackageName = "k8s.namespacelabs.dev/service-package-name"
	K8sServiceGrpcType    = "k8s.namespacelabs.dev/service-grpc-type"
	K8sEnvName            = "k8s.namespacelabs.dev/env"
	K8sEnvEphemeral       = "k8s.namespacelabs.dev/env-ephemeral"
	K8sEnvPurpose         = "k8s.namespacelabs.dev/env-purpose"
	K8sEnvTimeout         = "k8s.namespacelabs.dev/env-timeout"
	K8sNamespaceDriver    = "k8s.namespacelabs.dev/namespace-driver"
	K8sConfigImage        = "k8s.namespacelabs.dev/config-image"
	K8sKind               = "k8s.namespacelabs.dev/kind"
	K8sRuntimeConfig      = "k8s.namespacelabs.dev/runtime-config"

	K8sStaticConfigKind  = "static-config"
	K8sRuntimeConfigKind = "runtime-config"

	AppKubernetesIoManagedBy = "app.kubernetes.io/managed-by"

	ManagerId               = "foundation.namespace.so" // #220 Update when product name is final
	K8sFieldManager         = ManagerId
	defaultEphemeralTimeout = time.Hour
)

func SelectById(srv Deployable) map[string]string {
	return map[string]string{
		K8sServerId: srv.GetId(),
	}
}

func SelectEphemeral() map[string]string {
	return map[string]string{
		K8sEnvEphemeral: "true",
	}
}

func SelectNamespaceDriver() map[string]string {
	return map[string]string{
		K8sNamespaceDriver: "true",
	}
}

func SelectByPurpose(p schema.Environment_Purpose) map[string]string {
	return map[string]string{
		K8sEnvPurpose: strings.ToLower(p.String()),
	}
}

func ManagedByUs() map[string]string {
	return map[string]string{
		AppKubernetesIoManagedBy: ManagerId,
	}
}

type Deployable interface {
	GetId() string
	GetName() string
}

// Env may be nil.
func MakeLabels(env *schema.Environment, srv Deployable) map[string]string {
	// XXX add recommended labels https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	m := ManagedByUs()
	if srv != nil && srv.GetId() != "" {
		m[K8sServerId] = srv.GetId()
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

func WithFocusMark(labels map[string]string) map[string]string {
	labels[K8sServerFocus] = "true"
	return labels
}

func HasFocusMark(labels map[string]string) bool {
	label, ok := labels[K8sServerFocus]
	if !ok {
		return false
	}

	return label == "true"
}

// Env may be nil.
func MakeAnnotations(env *schema.Environment, pkg schema.PackageName) map[string]string {
	m := map[string]string{}

	if pkg != "" {
		m[K8sServerPackageName] = pkg.String()
	}

	if env.GetEphemeral() {
		m[K8sEnvTimeout] = defaultEphemeralTimeout.String()
	}

	// XXX add annotations with pointers to tools, team owners, etc.
	return m
}

func MakeServiceAnnotations(endpoint *schema.Endpoint) (map[string]string, error) {
	m := map[string]string{
		K8sServicePackageName: endpoint.GetEndpointOwner(),
	}

	var grpcServices []string
	for _, p := range endpoint.ServiceMetadata {
		if p.Protocol == schema.ClearTextGrpcProtocol || p.Protocol == schema.GrpcProtocol {
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

func MakeServiceLabels(env *schema.Environment, srv Deployable, endpoint *schema.Endpoint) map[string]string {
	m := MakeLabels(env, srv)

	return m
}
