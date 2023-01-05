// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/library/kubernetes/ingress"
	"namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &ingress.IngressIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		labels := kubedef.MakeLabels(intent.Env, intent.Deployable)

		source := &protocol.ResourceInstance{}
		if err := req.UnpackInput(source); err != nil {
			return err
		}

		instance := &runtime.IngressInstance{}
		for _, endpoint := range intent.Endpoint {
			if endpoint.Port == nil {
				// Maybe fail.
				continue
			}

			var domains []string
			for _, base := range intent.ApplicationBaseDomain {
				domains = append(domains, fmt.Sprintf("%s.%s", endpoint.AllocatedName, base))
			}
			domains = append(domains, endpoint.IngressSpec.GetDomain()...)

			if len(domains) == 0 {
				continue
			}

			name := endpoint.ServiceName

			backend := networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: endpoint.AllocatedName,
					Port: networkingv1.ServiceBackendPort{
						Number: endpoint.Port.ContainerPort,
					},
				},
			}

			for _, domain := range domains {
				var protocol *string
				var protocolDetails []*anypb.Any
				var httpExtensions []*anypb.Any
				for _, md := range endpoint.ServiceMetadata {
					if md.Protocol != "" {
						if protocol == nil {
							protocol = &md.Protocol
							if md.Details != nil {
								protocolDetails = append(protocolDetails, md.Details)
							}
						} else if *protocol != md.Protocol {
							return fnerrors.InternalError("%s: inconsistent protocol definition, %q and %q", endpoint.GetServiceName(), *protocol, md.Protocol)
						}
					}

					if md.Kind == "http-extension" && md.Details != nil {
						httpExtensions = append(httpExtensions, md.Details)
					}
				}

				if protocol == nil {
					continue
				}

				var spec networkingv1.IngressSpec
				var grpcCount, clearTextGrpcCount int
				switch *protocol {
				case schema.HttpProtocol:
					for _, details := range protocolDetails {
						p := &schema.HttpUrlMap{}
						if err := details.UnmarshalTo(p); err != nil {
							return err
						}

						var rules networkingv1.HTTPIngressRuleValue
						for _, entry := range p.Entry {
							rules.Paths = append(rules.Paths, makePathPrefix(entry.PathPrefix, backend))
						}
						spec.Rules = append(spec.Rules, makeRule(domain, rules))
					}

				case schema.GrpcProtocol, schema.ClearTextGrpcProtocol:
					for _, details := range protocolDetails {
						msg, err := details.UnmarshalNew()
						if err != nil {
							return fnerrors.InternalError("failed to unserialize grpc configuration: %w", err)
						}

						if *protocol == schema.ClearTextGrpcProtocol {
							clearTextGrpcCount++
						} else {
							grpcCount++
						}

						switch p := msg.(type) {
						case *schema.GrpcExportService:
							grpcService := p.ProtoTypename
							if grpcService == "" {
								return fnerrors.InternalError("%s: grpc service name is required", endpoint.ServiceName)
							}

							var rules networkingv1.HTTPIngressRuleValue
							for _, method := range p.Method {
								rules.Paths = append(rules.Paths, makePathPrefix(fmt.Sprintf("/%s/%s", grpcService, method), backend))
							}

							if len(rules.Paths) == 0 {
								rules.Paths = append(rules.Paths, makePathPrefix("/"+grpcService, backend))
							}

							if p.ServerReflectionIncluded {
								rules.Paths = append(rules.Paths, makePathPrefix("/grpc.reflection.v1alpha.ServerReflection", backend))
							}

							spec.Rules = append(spec.Rules, makeRule(domain, rules))

						case *schema.GrpcExportAllServices:
							var rules networkingv1.HTTPIngressRuleValue
							rules.Paths = append(rules.Paths, makePathPrefix("/", backend))
							spec.Rules = append(spec.Rules, makeRule(domain, rules))

						default:
							return fnerrors.InternalError("unsupported grpc configuration: %v", p.ProtoReflect().Descriptor().FullName())
						}
					}

				default:
					return fnerrors.New("%s: unsupported ingress protocol", *protocol)
				}

				if len(spec.Rules) == 0 {
					var rules networkingv1.HTTPIngressRuleValue
					rules.Paths = append(rules.Paths, makePathPrefix("/", backend))
					spec.Rules = append(spec.Rules, makeRule(domain, rules))
				}

				backendProtocol := "http"
				if clearTextGrpcCount > 0 {
					backendProtocol = "grpc"
				}
				if grpcCount > 0 {
					backendProtocol = "grpcs"
				}

				// XXX make nginx configurable.
				annotations, err := nginx.IngressAnnotations(false, backendProtocol, httpExtensions)
				if err != nil {
					return err
				}

				out.Invocations = append(out.Invocations, kubedef.Apply{
					Description:  fmt.Sprintf("Ingress %s", name),
					SetNamespace: true,
					Resource: networkingv1.Ingress{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Ingress",
							APIVersion: "networking.k8s.io/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:        name,
							Labels:      labels,
							Annotations: annotations,
						},
						Spec: spec,
					},
					SchedAfterCategory: []string{
						kubedef.MakeServicesCat(intent.Deployable),
					},
				})

				// A fragment is emited for visualization purposes.
				// XXX revisit "fragment" as an input, into strictly being an output.
				fragment := &schema.IngressFragment{
					Owner:    endpoint.EndpointOwner,
					Endpoint: endpoint,
					Domain: &schema.Domain{
						Fqdn:    domain,
						Managed: schema.Domain_LOCAL_MANAGED,
					},
				}

				for _, path := range spec.Rules {
					for _, y := range path.HTTP.Paths {
						fragment.HttpPath = append(fragment.HttpPath, &schema.IngressFragment_IngressHttpPath{
							Path:        y.Path,
							Owner:       endpoint.EndpointOwner,
							Service:     endpoint.AllocatedName,
							ServicePort: endpoint.GetExportedPort(),
						})
					}
				}

				instance.IngressFragment = append(instance.IngressFragment, fragment)
			}
		}

		out.OutputResourceInstance = instance
		return nil
	})
	provisioning.Handle(h)
}

func makeRule(domain string, rules networkingv1.HTTPIngressRuleValue) networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host:             domain,
		IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &rules},
	}
}

func makePathPrefix(path string, backend networkingv1.IngressBackend) networkingv1.HTTPIngressPath {
	x := networkingv1.PathTypePrefix
	return networkingv1.HTTPIngressPath{
		Path:     "/",
		PathType: &x,
		Backend:  backend,
	}
}
