// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eval

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

type PortAllocations struct {
	Ports []*schema.Endpoint_Port
}

type allocatedPort struct {
	Port int32 `json:"port"`
}

func MakePortAllocator(portBase int32, allocs *PortAllocations) AllocatorFunc {
	k := portBase

	return func(ctx context.Context, _ *schema.Node, n *schema.Need) (interface{}, error) {
		if p := n.GetPort(); p != nil {
			port := k
			k++

			allocs.Ports = append(allocs.Ports, &schema.Endpoint_Port{
				Name:          p.Name,
				ContainerPort: port,
			})

			return allocatedPort{Port: port}, nil
		}

		return nil, nil
	}
}