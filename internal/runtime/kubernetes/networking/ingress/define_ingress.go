// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applynetworkingv1 "k8s.io/client-go/applyconfigurations/networking/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
)

func PlanIngress(ctx context.Context, ingressPlanner kubedef.IngressClass, ns string, env *schema.Environment, deployable runtime.Deployable, fragments []*schema.IngressFragment) ([]defs.MakeDefinition, error) {
	var applies []defs.MakeDefinition

	groups := groupByName(fragments)

	slices.SortFunc(groups, func(a, b IngressGroup) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	for _, g := range groups {
		apply, err := generateForSrv(ctx, ingressPlanner, env, deployable, ns, g)
		if err != nil {
			return nil, err
		}

		applies = append(applies, apply...)
	}

	return applies, nil
}

type Cert struct {
	SecretName string
	Defs       []defs.MakeDefinition
}

func MakeCertificateSecrets(ns string, fragments []*schema.IngressFragment) map[string]Cert {
	domains := map[string]*schema.Domain{}
	domainCerts := map[string]*schema.Certificate{}

	for _, frag := range fragments {
		if frag.Domain != nil && frag.Domain.Fqdn != "" && frag.DomainCertificate != nil {
			// XXX check they're consistent.
			domains[frag.Domain.Fqdn] = frag.Domain
			domainCerts[frag.Domain.Fqdn] = frag.DomainCertificate
		}
	}

	certSecrets := map[string]Cert{} // Map fqdn->secret name.
	for _, domain := range domains {
		name := fmt.Sprintf("tls-%s", strings.ReplaceAll(domain.Fqdn, ".", "-"))
		cert := domainCerts[domain.Fqdn]
		certSecrets[domain.Fqdn] = Cert{
			SecretName: name,
			Defs: []defs.MakeDefinition{
				kubedef.Apply{
					Description: fmt.Sprintf("Certificate for %s", domain.Fqdn),
					Resource: applycorev1.
						Secret(name, ns).
						WithType(corev1.SecretTypeTLS).
						WithLabels(kubedef.ManagedByUs()).
						WithAnnotations(kubedef.BaseAnnotations()).
						WithData(map[string][]byte{
							"tls.key": cert.PrivateKey,
							"tls.crt": cert.CertificateBundle,
						}),
				},
			},
		}
	}

	return certSecrets
}

type IngressGroup struct {
	Name      string
	Fragments []*schema.IngressFragment
}

func groupByName(ngs []*schema.IngressFragment) []IngressGroup {
	sort.Slice(ngs, func(i, j int) bool {
		return strings.Compare(ngs[i].Name, ngs[j].Name) < 0
	})

	var groups []IngressGroup

	var currentName string
	var g []*schema.IngressFragment
	for _, ng := range ngs {
		if ng.Name != currentName {
			if len(g) > 0 {
				groups = append(groups, IngressGroup{
					Name:      g[0].Name,
					Fragments: g,
				})
			}
			g = nil
			currentName = ng.Name
		}

		g = append(g, ng)
	}
	if len(g) > 0 {
		groups = append(groups, IngressGroup{
			Name:      g[0].Name,
			Fragments: g,
		})
	}

	return groups
}

func generateForSrv(ctx context.Context, ingressPlanner kubedef.IngressClass, env *schema.Environment, deployable runtime.Deployable, ns string, g IngressGroup) ([]defs.MakeDefinition, error) {
	var clearTextGrpcCount, grpcCount, nonGrpcCount int

	certSecrets := MakeCertificateSecrets(ns, g.Fragments)
	labels := kubedef.MakeLabels(env, deployable)

	spec := applynetworkingv1.IngressSpec()

	var tlsCount int
	var extensions []*anypb.Any
	var applies []defs.MakeDefinition
	for _, ng := range g.Fragments {
		extensions = append(extensions, ng.Extension...)

		var paths []*applynetworkingv1.HTTPIngressPathApplyConfiguration
		for _, p := range ng.HttpPath {
			nonGrpcCount++

			if p.ServicePort == 0 {
				return nil, fnerrors.InternalError("%s: ingress definition without port", filepath.Join(p.Path, p.Service))
			}

			// XXX validate ports.
			paths = append(paths, applynetworkingv1.HTTPIngressPath().
				WithPath(p.Path).
				WithPathType(netv1.PathTypePrefix).
				WithBackend(
					applynetworkingv1.IngressBackend().WithService(
						applynetworkingv1.IngressServiceBackend().WithName(p.Service).WithPort(
							applynetworkingv1.ServiceBackendPort().WithNumber(p.ServicePort)))))
		}

		for _, p := range ng.GrpcService {
			if p.BackendTls {
				grpcCount++
			} else {
				clearTextGrpcCount++
			}

			if p.ServicePort == 0 {
				return nil, fnerrors.InternalError("%s: ingress definition without port", filepath.Join(p.GrpcService, p.Service))
			}

			if p.GrpcService == "" && !p.AllServices {
				return nil, fnerrors.InternalError("%s: grpc service name is required", p.Service)
			}

			backend := applynetworkingv1.IngressBackend().
				WithService(applynetworkingv1.IngressServiceBackend().
					WithName(p.Service).
					WithPort(applynetworkingv1.ServiceBackendPort().WithNumber(p.ServicePort)))

			if len(p.Method) == 0 {
				paths = append(paths, applynetworkingv1.HTTPIngressPath().
					WithPath("/"+p.GrpcService).
					WithPathType(netv1.PathTypePrefix).
					WithBackend(backend))
			} else {
				for _, method := range p.Method {
					paths = append(paths, applynetworkingv1.HTTPIngressPath().
						WithPath(fmt.Sprintf("/%s/%s", p.GrpcService, method)).
						WithPathType(netv1.PathTypeExact).
						WithBackend(backend))
				}
			}
		}

		spec = spec.WithRules(applynetworkingv1.IngressRule().WithHost(ng.Domain.Fqdn).WithHTTP(
			applynetworkingv1.HTTPIngressRuleValue().WithPaths(
				paths...)))

		if tlsSecret, ok := certSecrets[ng.Domain.Fqdn]; ok {
			spec = spec.WithTLS(applynetworkingv1.IngressTLS().WithHosts(ng.Domain.Fqdn).WithSecretName(tlsSecret.SecretName))
			applies = append(applies, tlsSecret.Defs...)
			tlsCount++
		}

		ops, err := ingressPlanner.Map(ctx, ng.Domain, ns, g.Name)
		if err != nil {
			return nil, err
		}

		for _, frag := range ops {
			desc := frag.Description
			if desc == "" {
				desc = fmt.Sprintf("Update %s's address", frag.Fdqn)
			}

			applies = append(applies, defs.Static(desc, frag))
		}
	}

	if grpcCount > 0 && nonGrpcCount > 0 {
		return nil, fnerrors.InternalError("can't mix grpc and non-grpc backends in the same ingress")
	}

	if grpcCount > 0 && clearTextGrpcCount > 0 {
		return nil, fnerrors.InternalError("can't mix grpc and cleartext-grpc backends in the same ingress")
	}

	backendProtocol := "http"
	if clearTextGrpcCount > 0 {
		backendProtocol = "grpc"
	}
	if grpcCount > 0 {
		backendProtocol = "grpcs"
	}

	// XXX make nginx configurable.
	annotations, err := nginx.IngressAnnotations(tlsCount > 0, backendProtocol, extensions)
	if err != nil {
		return nil, err
	}

	applies = append(applies, kubedef.Apply{
		Description: fmt.Sprintf("Ingress %s", g.Name),
		Resource: applynetworkingv1.Ingress(g.Name, ns).
			WithLabels(labels).
			WithAnnotations(annotations).
			WithSpec(spec),
		SchedAfterCategory: []string{
			kubedef.MakeServicesCat(deployable),
		},
	})

	return applies, nil
}

func Delete(ns string, stack []planning.Server) ([]*schema.SerializedInvocation, error) {
	var defs []*schema.SerializedInvocation

	for _, srv := range stack {
		op := kubedef.DeleteList{
			Description: "Ingresses",
			Resource:    "ingresses",
			Namespace:   ns,
			Selector:    kubedef.SelectById(srv.Proto()),
		}

		if def, err := op.ToDefinition(srv.PackageName()); err != nil {
			return nil, err
		} else {
			defs = append(defs, def)
		}
	}

	return defs, nil
}
