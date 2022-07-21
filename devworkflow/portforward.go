// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"io"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func NewPortFwd(ctx context.Context, obs *Session, selector runtime.Selector, localaddr string) *endpointfwd.PortForward {
	pfw := &endpointfwd.PortForward{
		Env:       selector.Proto(),
		LocalAddr: localaddr,
		Debug:     console.Debug(ctx),
		Warnings:  console.Warnings(ctx),
		ForwardPort: func(server *schema.Server, port int32, localAddr []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
			return runtime.For(ctx, selector).ForwardPort(ctx, server, port, localAddr, callback)
		},
		ForwardIngress: func(localAddr []string, port int, callback runtime.PortForwardedFunc) (io.Closer, error) {
			return runtime.For(ctx, selector).ForwardIngress(ctx, localAddr, port, callback)
		},
	}

	if obs != nil {
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
			obs.updateStackInPlace(func(stack *Stack) { stack.NetworkPlan = pfw.ToNetworkPlan() })
		}
	}

	return pfw
}
