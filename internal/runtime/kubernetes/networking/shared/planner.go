// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"context"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

func PrepareRoute(ctx context.Context, env *schema.Environment, srv *schema.Stack_Entry, domain *schema.Domain, ns, name string, ingressSvc *kubedef.OpMapAddress_ServiceRef) (*kubedef.IngressAllocatedRoute, error) {
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
