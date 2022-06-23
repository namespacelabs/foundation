// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func newPortFwd(obs *Session, selector runtime.Selector, localaddr string) *endpointfwd.PortForward {
	pfw := &endpointfwd.PortForward{
		Selector:  selector,
		LocalAddr: localaddr,
	}

	pfw.OnAdd = func(endpoint *schema.Endpoint, localPort uint) {
		obs.updateStackInPlace(func(stack *Stack) {
			for _, fwd := range stack.ForwardedPort {
				if proto.Equal(fwd.Endpoint, endpoint) {
					fwd.LocalPort = int32(localPort)
					return
				}
			}

			stack.ForwardedPort = append(stack.ForwardedPort, &ForwardedPort{
				Endpoint:      endpoint,
				ContainerPort: endpoint.GetPort().GetContainerPort(),
				LocalPort:     int32(localPort),
			})
		})
	}

	pfw.OnDelete = func(unused []*schema.Endpoint) {
		obs.updateStackInPlace(func(stack *Stack) {
			var portFwds []*ForwardedPort
			for _, fwd := range stack.ForwardedPort {
				filtered := false
				for _, endpoint := range unused {
					if fwd.Endpoint == endpoint {
						filtered = true
						break
					}
				}
				if !filtered {
					portFwds = append(portFwds, fwd)
				}
			}

			stack.ForwardedPort = portFwds
		})
	}

	pfw.OnUpdate = func() {
		obs.setSticky(pfw.Render(colors.WithColors))
	}

	return pfw
}
