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
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
)

func NewPortFwd(ctx context.Context, obs *Session, env planning.Context, localaddr string) *endpointfwd.PortForward {
	pfw := &endpointfwd.PortForward{
		Env:       env.Environment(),
		LocalAddr: localaddr,
		Debug:     console.Debug(ctx),
		Warnings:  console.Warnings(ctx),
		ForwardPort: func(server *schema.Server, port int32, localAddr []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
			rt, err := runtime.ClusterFor(ctx, env)
			if err != nil {
				return nil, err
			}

			return rt.ForwardPort(ctx, server, port, localAddr, callback)
		},
		ForwardIngress: func(localAddr []string, port int, callback runtime.PortForwardedFunc) (io.Closer, error) {
			rt, err := runtime.ClusterFor(ctx, env)
			if err != nil {
				return nil, err
			}

			return rt.ForwardIngress(ctx, localAddr, port, callback)
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

		pfw.OnUpdate = func(plan *storage.NetworkPlan) {
			obs.updateStackInPlace(func(stack *Stack) { stack.NetworkPlan = plan })
		}
	}

	return pfw
}
