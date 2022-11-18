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
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
)

const LblNameStatus = "ns:kubernetes:ingress:status"

func EnsureStack(ctx context.Context) ([]*schema.SerializedInvocation, error) {
	// XXX make this configurable.
	return nginx.Ensure(ctx)
}

type MapAddress struct {
	FQDN    string
	Ingress IngressRef
}

type IngressRef struct {
	Namespace, Name string
}

func PlanIngress(ctx context.Context, ns string, env *schema.Environment, deployable runtime.Deployable, fragments []*schema.IngressFragment, certSecrets map[string]string) ([]kubedef.Apply, *MapAddressList, error) {
	var applies []kubedef.Apply

	groups := groupByName(fragments)

	var allManaged MapAddressList
	for _, g := range groups {
		apply, managed, err := generateForSrv(ctx, ns, env, deployable, g.Name, g.Fragments, certSecrets)
		if err != nil {
			return nil, nil, err
		}

		applies = append(applies, kubedef.Apply{
			Description: fmt.Sprintf("Ingress %s", g.Name),
			Resource:    apply,
			SchedAfterCategory: []string{
				kubedef.MakeServicesCat(deployable),
			},
		})

		if err := allManaged.Merge(managed); err != nil {
			return nil, nil, err
		}
	}

	// Since we built the Cert list from a map, it's order is non-deterministic.
	sort.Slice(applies, func(i, j int) bool {
		return strings.Compare(applies[i].Description, applies[j].Description) < 0
	})

	return applies, &allManaged, nil
}

func MakeCertificateSecrets(ns string, fragments []*schema.IngressFragment) (map[string]string, []kubedef.Apply) {
	var applies []kubedef.Apply

	domains := map[string]*schema.Domain{}
	domainCerts := map[string]*schema.Certificate{}

	for _, frag := range fragments {
		if frag.Domain != nil && frag.Domain.Fqdn != "" && frag.DomainCertificate != nil {
			// XXX check they're consistent.
			domains[frag.Domain.Fqdn] = frag.Domain
			domainCerts[frag.Domain.Fqdn] = frag.DomainCertificate
		}
	}

	certSecrets := map[string]string{} // Map fqdn->secret name.
	for _, domain := range domains {
		name := fmt.Sprintf("tls-%s", strings.ReplaceAll(domain.Fqdn, ".", "-"))
		certSecrets[domain.Fqdn] = name
		cert := domainCerts[domain.Fqdn]
		applies = append(applies, kubedef.Apply{
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
		})
	}

	// Since we built the Cert list from a map, it's order is non-deterministic.
	sort.Slice(applies, func(i, j int) bool {
		return strings.Compare(applies[i].Description, applies[j].Description) < 0
	})

	return certSecrets, applies
}

type ingressGroup struct {
	Name      string
	Fragments []*schema.IngressFragment
}

func groupByName(ngs []*schema.IngressFragment) []ingressGroup {
	sort.Slice(ngs, func(i, j int) bool {
		return strings.Compare(ngs[i].Name, ngs[j].Name) < 0
	})

	var groups []ingressGroup

	var currentName string
	var g []*schema.IngressFragment
	for _, ng := range ngs {
		if ng.Name != currentName {
			if len(g) > 0 {
				groups = append(groups, ingressGroup{
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
		groups = append(groups, ingressGroup{
			Name:      g[0].Name,
			Fragments: g,
		})
	}

	return groups
}

type MapAddressList struct {
	index   map[string]MapAddress // fqdn -> MapAddress
	sources map[string][]string   // fqdn -> list of sources
}

func (m *MapAddressList) Add(ma MapAddress, sources ...string) error {
	if existing, has := m.index[ma.FQDN]; has {
		if existing.Ingress.Name != ma.Ingress.Name || existing.Ingress.Namespace != ma.Ingress.Namespace {
			return fnerrors.InternalError("%s: incompatible map address definitions: %s/%s (from %s) vs %s/%s",
				ma.FQDN, existing.Ingress.Namespace, existing.Ingress.Name,
				strings.Join(m.sources[ma.FQDN], ", "), ma.Ingress.Namespace, ma.Ingress.Name)
		}

		return nil
	}

	if m.index == nil {
		m.index = map[string]MapAddress{}
		m.sources = map[string][]string{}
	}

	m.index[ma.FQDN] = ma
	m.sources[ma.FQDN] = append(m.sources[ma.FQDN], sources...)

	return nil
}

func (m *MapAddressList) Merge(rhs *MapAddressList) error {
	if m == nil {
		return nil
	}

	var errs []error
	for _, ma := range rhs.index {
		errs = append(errs, m.Add(ma, rhs.sources[ma.FQDN]...))
	}
	return multierr.New(errs...)
}

func (m *MapAddressList) Sorted() []MapAddress {
	var mas []MapAddress
	for _, ma := range m.index {
		mas = append(mas, ma)
	}
	slices.SortFunc(mas, func(a, b MapAddress) bool {
		return strings.Compare(a.FQDN, b.FQDN) < 0
	})
	return mas
}

func generateForSrv(ctx context.Context, ns string, env *schema.Environment, srv runtime.Deployable, name string, fragments []*schema.IngressFragment, certSecrets map[string]string) (*applynetworkingv1.IngressApplyConfiguration, *MapAddressList, error) {
	var clearTextGrpcCount, grpcCount, nonGrpcCount int

	spec := applynetworkingv1.IngressSpec()

	var tlsCount int
	var managed MapAddressList
	var extensions []*anypb.Any
	for _, ng := range fragments {
		extensions = append(extensions, ng.Extension...)

		var paths []*applynetworkingv1.HTTPIngressPathApplyConfiguration
		for _, p := range ng.HttpPath {
			nonGrpcCount++

			if p.Port == nil {
				return nil, nil, fnerrors.InternalError("%s: ingress definition without port", filepath.Join(p.Path, p.Service))
			}

			// XXX validate ports.
			paths = append(paths, applynetworkingv1.HTTPIngressPath().
				WithPath(p.Path).
				WithPathType(netv1.PathTypePrefix).
				WithBackend(
					applynetworkingv1.IngressBackend().WithService(
						applynetworkingv1.IngressServiceBackend().WithName(p.Service).WithPort(
							applynetworkingv1.ServiceBackendPort().WithNumber(p.Port.ContainerPort)))))
		}

		for _, p := range ng.GrpcService {
			if p.BackendTls {
				grpcCount++
			} else {
				clearTextGrpcCount++
			}

			if p.Port == nil {
				return nil, nil, fnerrors.InternalError("%s: ingress definition without port", filepath.Join(p.GrpcService, p.Service))
			}

			if p.GrpcService == "" && !p.AllServices {
				return nil, nil, fnerrors.InternalError("%s: grpc service name is required", p.Service)
			}

			backend := applynetworkingv1.IngressBackend().
				WithService(applynetworkingv1.IngressServiceBackend().
					WithName(p.Service).
					WithPort(applynetworkingv1.ServiceBackendPort().WithNumber(p.Port.ContainerPort)))

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
			spec = spec.WithTLS(applynetworkingv1.IngressTLS().WithHosts(ng.Domain.Fqdn).WithSecretName(tlsSecret))
			tlsCount++
		}

		if ng.Domain.Managed == schema.Domain_CLOUD_MANAGED && ng.Domain.TlsInclusterTermination {
			if err := managed.Add(MapAddress{
				FQDN:    ng.Domain.Fqdn,
				Ingress: IngressRef{ns, name},
			}, name); err != nil {
				return nil, nil, err
			}
		}
	}

	if grpcCount > 0 && nonGrpcCount > 0 {
		return nil, nil, fnerrors.InternalError("can't mix grpc and non-grpc backends in the same ingress")
	}

	if grpcCount > 0 && clearTextGrpcCount > 0 {
		return nil, nil, fnerrors.InternalError("can't mix grpc and cleartext-grpc backends in the same ingress")
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
		return nil, nil, err
	}

	ingress := applynetworkingv1.Ingress(name, ns).
		WithLabels(kubedef.MakeLabels(env, srv)).
		WithAnnotations(annotations).
		WithSpec(spec)

	return ingress, &managed, nil
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
