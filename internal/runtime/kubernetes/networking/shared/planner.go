// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"context"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

type MapPublicLoadBalancer struct{}

func (MapPublicLoadBalancer) Map(ctx context.Context, domain *schema.Domain, ns, name string) ([]*kubedef.OpMapAddress, error) {
	if domain.Managed == schema.Domain_CLOUD_MANAGED && domain.TlsInclusterTermination {
		return []*kubedef.OpMapAddress{{
			Fdqn:        domain.Fqdn,
			IngressNs:   ns,
			IngressName: name,
		}}, nil
	}

	return nil, nil
}
