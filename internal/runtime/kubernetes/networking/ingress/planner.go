// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"context"
)

type MapPublicLoadBalancer struct{}

type Planner interface {
	Map(context.Context, MapAddress) ([]*OpMapAddress, error)
}

func (MapPublicLoadBalancer) Map(ctx context.Context, frag MapAddress) ([]*OpMapAddress, error) {
	return []*OpMapAddress{{
		Fdqn:        frag.FQDN,
		IngressNs:   frag.Ingress.Namespace,
		IngressName: frag.Ingress.Name,
	}}, nil
}
