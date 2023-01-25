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
	netv1 "k8s.io/api/networking/v1"
	applynetworkingv1 "k8s.io/client-go/applyconfigurations/networking/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
)

const IngressControllerCat = "kube:ingress:controller"

func PlanIngress(ctx context.Context, ingressPlanner kubedef.IngressClass, ns string, env *schema.Environment, srv *schema.Stack_Entry, fragments []*schema.IngressFragment) ([]defs.MakeDefinition, error) {
	var applies []defs.MakeDefinition

	groups := groupByName(fragments)

	slices.SortFunc(groups, func(a, b IngressGroup) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	for _, g := range groups {
		apply, err := generateForSrv(ctx, ingressPlanner, env, srv, ns, g)
		if err != nil {
			return nil, err
		}

		applies = append(applies, apply...)
	}

	return applies, nil
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

func generateForSrv(ctx context.Context, ingress kubedef.IngressClass, env *schema.Environment, srv *schema.Stack_Entry, ns string, g IngressGroup) ([]defs.MakeDefinition, error) {
	var clearTextGrpcCount, grpcCount, nonGrpcCount int

	labels := kubedef.MakeLabels(env, srv.Server)

	spec := applynetworkingv1.IngressSpec()

	var tlsCount int
	var extensions []*anypb.Any
	var applies []defs.MakeDefinition
	var domains []*schema.Domain
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

		domains = append(domains, ng.Domain)

		route, err := ingress.PrepareRoute(ctx, env, srv, ng.Domain, ns, g.Name)
		if err != nil {
			return nil, err
		}

		if route != nil {
			for _, frag := range route.Map {
				desc := frag.Description
				if desc == "" {
					desc = fmt.Sprintf("Update %s's address", frag.Fdqn)
				}

				applies = append(applies, defs.Static(desc, frag))
			}

			if tlsSecret, ok := route.Certificates[ng.Domain.Fqdn]; ok {
				spec = spec.WithTLS(applynetworkingv1.IngressTLS().WithHosts(ng.Domain.Fqdn).WithSecretName(tlsSecret.SecretName))
				applies = append(applies, tlsSecret.Defs...)
				tlsCount++
			}
		}
	}

	if grpcCount > 0 && nonGrpcCount > 0 {
		return nil, fnerrors.InternalError("can't mix grpc and non-grpc backends in the same ingress")
	}

	if grpcCount > 0 && clearTextGrpcCount > 0 {
		return nil, fnerrors.InternalError("can't mix grpc and cleartext-grpc backends in the same ingress")
	}

	backendProtocol := kubedef.BackendProtocol_HTTP
	if clearTextGrpcCount > 0 {
		backendProtocol = kubedef.BackendProtocol_GRPC
	}
	if grpcCount > 0 {
		backendProtocol = kubedef.BackendProtocol_GRPCS
	}

	annotations, err := ingress.Annotate(ns, g.Name, domains, tlsCount > 0, backendProtocol, extensions)
	if err != nil {
		return nil, err
	}

	applies = append(applies, kubedef.Apply{
		Description: fmt.Sprintf("Ingress %s", g.Name),
		Resource: applynetworkingv1.Ingress(g.Name, ns).
			WithLabels(labels).
			WithAnnotations(annotations.Annotations).
			WithSpec(spec),
		SchedAfterCategory: append([]string{
			kubedef.MakeServicesCat(srv.Server),
			IngressControllerCat,
		}, annotations.SchedAfter...),
	})

	applies = append(applies, annotations.Resources...)

	return applies, nil
}
