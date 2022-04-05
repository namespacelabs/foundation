// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

const fetchLogsAfter = 10 * time.Second

type updateCluster struct {
	obs       *stackState
	localAddr string
	env       ops.WorkspaceEnvironment
	stack     *schema.Stack
	servers   []provision.Server
	focus     []schema.PackageName
	observers []languages.DevObserver

	plan compute.Computable[*deploy.Plan]

	mu               sync.Mutex
	done             bool
	revision         int
	endpointPortFwds map[string]*portFwd
	ingressPortfwd   portFwd
	domains          []*runtime.FilteredDomain
}

type portFwd struct {
	endpoint  *schema.Endpoint
	closer    io.Closer
	err       error
	revision  int
	localPort uint
}

func (pi *updateCluster) Inputs() *compute.In {
	return compute.Inputs().Computable("plan", pi.plan)
}

func (pi *updateCluster) Updated(ctx context.Context, deps compute.Resolved) error {
	plan := compute.GetDepValue(deps, pi.plan, "plan")

	waiters, err := plan.Deployer.Apply(ctx, runtime.TaskServerDeploy, pi.env)
	if err != nil {
		return err
	}

	if err := deploy.Wait(ctx, pi.servers, pi.env, waiters); err != nil {
		return err
	}

	pi.mu.Lock()
	defer pi.mu.Unlock()

	for _, obs := range pi.observers {
		obs.OnDeployment()
	}

	domains, err := runtime.FilterAndDedupDomains(plan.IngressFragments, func(d *schema.Domain) bool {
		return d.GetManaged() != schema.Domain_MANAGED_UNKNOWN
	})

	if err == nil {
		pi.domains = domains
	} else {
		fmt.Fprintln(console.Stderr(ctx), "Failed to forward resulting ingress:", err)
	}

	if pi.endpointPortFwds == nil {
		pi.endpointPortFwds = map[string]*portFwd{}
	}

	pi.revision++

	for _, endpoint := range pi.stack.Endpoint {
		key := fmt.Sprintf("%s/%s/%s", endpoint.ServerOwner, endpoint.EndpointOwner, endpoint.ServiceName)
		if existing, ok := pi.endpointPortFwds[key]; ok {
			if proto.Equal(existing.endpoint, endpoint) {
				existing.revision = pi.revision
				continue
			}

			existing.closer.Close()
			delete(pi.endpointPortFwds, key)
		}

		instance := &portFwd{endpoint: endpoint, revision: pi.revision}

		endpoint := endpoint // Close endpoint.
		closer, err := pi.portFwd(ctx, endpoint, pi.revision, func(wasrevision int, localPort uint) {
			// Emit stack update without locks.
			if endpoint.GetPort().GetContainerPort() > 0 {
				pi.obs.updateStack(func(stack *Stack) *Stack {
					for _, fwd := range stack.ForwardedPort {
						if fwd.Endpoint == endpoint {
							fwd.LocalPort = int32(localPort)
							return stack
						}
					}

					stack.ForwardedPort = append(stack.ForwardedPort, &ForwardedPort{
						Endpoint:      endpoint,
						ContainerPort: endpoint.GetPort().GetContainerPort(),
						LocalPort:     int32(localPort),
					})
					return stack
				})
			}

			pi.mu.Lock()
			defer pi.mu.Unlock()

			instance.localPort = localPort
			if wasrevision == pi.revision {
				pi.redrawSticky()
			}
		})

		instance.closer = closer
		instance.err = err

		pi.endpointPortFwds[key] = instance
	}

	var unused []string
	for key, fwd := range pi.endpointPortFwds {
		if fwd.revision != pi.revision {
			// Endpoint no longer present.
			if fwd.closer != nil {
				fwd.closer.Close()
			}
			unused = append(unused, key)
		}
	}

	if len(unused) > 0 {
		pi.obs.updateStack(func(stack *Stack) *Stack {
			var portFwds []*ForwardedPort
			for _, fwd := range stack.ForwardedPort {
				filtered := false
				for _, key := range unused {
					if fwd.Endpoint == pi.endpointPortFwds[key].endpoint {
						filtered = true
						break
					}
				}
				if !filtered {
					portFwds = append(portFwds, fwd)
				}
			}

			stack.ForwardedPort = portFwds
			return stack
		})

		for _, key := range unused {
			delete(pi.endpointPortFwds, key)
		}
	}

	if len(domains) > 0 && pi.env.Proto().Purpose == schema.Environment_DEVELOPMENT {
		if pi.ingressPortfwd.closer == nil {
			pi.ingressPortfwd.closer, pi.ingressPortfwd.err = runtime.For(pi.env).ForwardIngress(ctx, []string{pi.localAddr}, runtime.LocalIngressPort, func(fpe runtime.ForwardedPortEvent) {
				pi.mu.Lock()
				defer pi.mu.Unlock()

				for _, port := range fpe.Added {
					// We should never receive multiple ports.
					pi.ingressPortfwd.endpoint = fpe.Endpoint
					pi.ingressPortfwd.localPort = port.LocalPort
				}

				pi.redrawSticky()
			})
		}
	} else if pi.ingressPortfwd.closer != nil {
		pi.ingressPortfwd.closer.Close()
		pi.ingressPortfwd.closer = nil
	}

	pi.redrawSticky()

	return nil
}

func (pi *updateCluster) redrawSticky() {
	if pi.done {
		return
	}

	var out bytes.Buffer

	var portFwds []*deploy.PortFwd

	for _, fwd := range pi.endpointPortFwds {
		portFwds = append(portFwds, &deploy.PortFwd{
			Endpoint:  fwd.endpoint,
			LocalPort: fwd.localPort,
		})
	}

	if pi.ingressPortfwd.endpoint != nil {
		portFwds = append(portFwds, &deploy.PortFwd{
			Endpoint:  pi.ingressPortfwd.endpoint,
			LocalPort: pi.ingressPortfwd.localPort,
		})
	}

	focus := focusServers(pi.stack, pi.focus)

	deploy.SortPorts(portFwds, focus)

	deploy.RenderPortsAndIngresses(true, &out, pi.localAddr, pi.stack, focus, portFwds, pi.domains, nil)

	pi.obs.parent.setSticky(out.Bytes())
}

func (pi *updateCluster) portFwd(ctx context.Context, endpoint *schema.Endpoint, revision int, callback func(int, uint)) (io.Closer, error) {
	server := pi.stack.GetServer(schema.PackageName(endpoint.ServerOwner))
	if server == nil {
		return nil, fnerrors.UserError(nil, "%s: missing in the stack", endpoint.ServerOwner)
	}

	return runtime.For(pi.env).ForwardPort(ctx, server.Server, endpoint, []string{pi.localAddr}, func(fp runtime.ForwardedPort) {
		callback(revision, fp.LocalPort)
	})
}

func (pi *updateCluster) Cleanup(_ context.Context) error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	pi.done = true

	for _, fwd := range pi.endpointPortFwds {
		if fwd.closer != nil {
			fwd.closer.Close()
		}
	}

	if pi.ingressPortfwd.closer != nil {
		pi.ingressPortfwd.closer.Close()
	}

	return nil
}
