// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eval

import (
	"context"
	"fmt"

	"github.com/cespare/xxhash/v2"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type PortAllocations struct {
	Ports []*schema.Endpoint_Port
}

type allocatedPort struct {
	Port int32 `json:"port"`
}

type PortRange struct {
	Base, Max int32
}

func DefaultPortRange() PortRange { return PortRange{40000, 41000} }

func MakePortAllocator(server *schema.Server, portRange PortRange, allocs *PortAllocations) AllocatorFunc {
	return func(ctx context.Context, _ *schema.Node, n *schema.Need) (interface{}, error) {
		if p := n.GetPort(); p != nil {
			const maxRounds = 10
			// We allocate ports based on the hash of the server name to
			// minimize collisions between ports of different servers, in the
			// same stack. This becomes important when forwarding ports back to
			// the localhost, as we need to project all of these into a single
			// namespace.
			for i := 0; i < maxRounds; i++ {
				hashInput := fmt.Sprintf("%s/%s/%d", server.PackageName, p.Name, i)
				hash := xxhash.Sum64([]byte(hashInput))
				port := portRange.Base + int32(hash%uint64(portRange.Max-portRange.Base))

				exists := false
				for _, allocated := range allocs.Ports {
					if allocated.ContainerPort == port {
						exists = true
						break
					}
				}

				if !exists {
					allocs.Ports = append(allocs.Ports, &schema.Endpoint_Port{
						Name:          p.Name,
						ContainerPort: port,
					})

					return allocatedPort{Port: port}, nil
				}
			}

			return nil, fnerrors.InternalError("%s/%s: failed to allocate port within %d rounds", server.PackageName, p.Name, maxRounds)
		}

		return nil, nil
	}
}
