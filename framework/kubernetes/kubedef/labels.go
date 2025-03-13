// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	"namespacelabs.dev/foundation/framework/kubernetes/kubenaming"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
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
	K8sEnvTimeout         = "k8s.namespacelabs.dev/env-timeout"
	K8sNamespaceDriver    = "k8s.namespacelabs.dev/namespace-driver"
	K8sConfigImage        = "k8s.namespacelabs.dev/config-image"
	K8sKind               = "k8s.namespacelabs.dev/kind"
	K8sRuntimeConfig      = "k8s.namespacelabs.dev/runtime-config"
	K8sPlannerVersion     = "k8s.namespacelabs.dev/planner-version"

	K8sStaticConfigKind  = "static-config"
	K8sRuntimeConfigKind = "runtime-config"

	AppKubernetesIoManagedBy = "app.kubernetes.io/managed-by"
	KubernetesIoArch         = "kubernetes.io/arch"

	ManagerId               = "foundation.namespace.so" // #220 Update when product name is final
	K8sFieldManager         = ManagerId
	defaultEphemeralTimeout = time.Hour

	PlannerVersion = 1
)

func SelectById(srv runtime.Deployable) map[string]string {
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

func ServerPackageLabels(srv runtime.Deployable) map[string]string {
	m := map[string]string{}
	if pkg := srv.GetPackageRef().GetPackageName(); pkg != "" {
		m[K8sServerPackageName] = kubenaming.LabelLike(pkg)
	}
	return m
}

// Env may be nil; srv may be nil.
func MakeLabels(env *schema.Environment, srv runtime.Deployable) map[string]string {
	// XXX add recommended labels https://kubernetes.io/docs/concepts/overview/working-with-objects/common-labels/
	m := ManagedByUs()
	if srv != nil && srv.GetId() != "" {
		m[K8sServerId] = srv.GetId()
	}
	if env != nil {
		m[K8sEnvPurpose] = strings.ToLower(env.Purpose.String())
		if env.Ephemeral {
			m[K8sEnvEphemeral] = "true"
		} else {
			m[K8sEnvName] = env.Name
			m[K8sEnvEphemeral] = "false"
		}
	}

	if srv != nil {
		maps.Copy(m, ServerPackageLabels(srv))
	}

	return m
}

func BaseAnnotations() map[string]string {
	return map[string]string{
		K8sPlannerVersion: fmt.Sprintf("%d", PlannerVersion),
	}
}

// Env may be nil.
func MakeAnnotations(env *schema.Environment) map[string]string {
	m := BaseAnnotations()

	if env.GetEphemeral() {
		m[K8sEnvTimeout] = defaultEphemeralTimeout.String()
	}

	// XXX add annotations with pointers to tools, team owners, etc.
	return m
}

func MakeServiceAnnotations(endpoint *schema.Endpoint) (map[string]string, error) {
	m := BaseAnnotations()

	m[K8sServicePackageName] = endpoint.GetEndpointOwner()

	var grpcServices []string
	for _, p := range endpoint.ServiceMetadata {
		if p.Protocol == schema.ClearTextGrpcProtocol || p.Protocol == schema.GrpcProtocol {
			if p.Details == nil || p.Details.MessageIs(&schema.GrpcExportAllServices{}) {
				continue
			}

			grpc := &schema.GrpcExportService{}
			if err := p.Details.UnmarshalTo(grpc); err != nil {
				return nil, fnerrors.Newf("failed to unserialize grpc configuration: %w", err)
			}

			grpcServices = append(grpcServices, grpc.ProtoTypename)
		}
	}

	if len(grpcServices) > 0 {
		m[K8sServiceGrpcType] = strings.Join(grpcServices, ",")
	}

	return m, nil
}

func MakeServiceLabels(env *schema.Environment, srv runtime.Deployable, endpoint *schema.Endpoint) map[string]string {
	m := MakeLabels(env, srv)

	return m
}
