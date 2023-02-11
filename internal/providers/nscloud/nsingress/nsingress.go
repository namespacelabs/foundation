// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nsingress

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
)

const (
	LocalBaseDomain = "nslocal.host"
	CloudBaseDomain = "nscloud.dev"
)

type Ingress struct {
	nginx.Ingress
}

func IngressClass() kubedef.IngressClass {
	return Ingress{}
}

func (Ingress) Name() string { return "nsingress-nginx" }

func (Ingress) ComputeNaming(env *schema.Environment, source *schema.Naming) (*schema.ComputedNaming, error) {
	if env.Purpose != schema.Environment_PRODUCTION {
		return &schema.ComputedNaming{
			Source:     source,
			BaseDomain: LocalBaseDomain,
			Managed:    schema.Domain_LOCAL_MANAGED,
		}, nil
	}

	if !source.GetEnableNamespaceManaged() {
		return &schema.ComputedNaming{}, nil
	}

	org := source.GetWithOrg()
	if org == "" {
		return &schema.ComputedNaming{}, nil
	}

	return &schema.ComputedNaming{
		Source:     source,
		BaseDomain: fmt.Sprintf("%s.%s", org, CloudBaseDomain),
		Managed:    schema.Domain_CLOUD_MANAGED,
	}, nil
}

func (n Ingress) PrepareRoute(ctx context.Context, env *schema.Environment, srv *schema.Stack_Entry, domain *schema.Domain, ns, name string) (*kubedef.IngressAllocatedRoute, error) {
	return prepareRoute(ctx, env, srv, domain, ns, name, &kubedef.OpMapAddress_ServiceRef{
		Namespace:   n.Service().Namespace,
		ServiceName: n.Service().ServiceName,
	})
}

func prepareRoute(ctx context.Context, env *schema.Environment, srv *schema.Stack_Entry, domain *schema.Domain, ns, name string, ingressSvc *kubedef.OpMapAddress_ServiceRef) (*kubedef.IngressAllocatedRoute, error) {
	var route kubedef.IngressAllocatedRoute

	if domain.Managed == schema.Domain_CLOUD_MANAGED || domain.Managed == schema.Domain_USER_SPECIFIED_TLS_MANAGED {
		cert, err := AllocateDomainCertificate(ctx, env, srv, domain)
		if err != nil {
			return nil, err
		}

		route.Certificates = MakeCertificateSecrets(ns, domain, cert)
	}

	if domain.Managed == schema.Domain_CLOUD_MANAGED {
		route.Map = []*kubedef.OpMapAddress{{
			Fdqn:           domain.Fqdn,
			IngressNs:      ns,
			IngressName:    name,
			IngressService: ingressSvc,
		}}
	}

	return &route, nil
}
