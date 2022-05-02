// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package endpointfwd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

type PortForward struct {
	LocalAddr string
	Env       ops.Environment
	Stack     *schema.Stack
	Focus     []schema.PackageName

	OnAdd    func(*schema.Endpoint, uint)
	OnDelete func([]*schema.Endpoint)
	OnUpdate func()

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

func (pi *PortForward) Update(ctx context.Context, fragments []*schema.IngressFragment) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	domains, err := runtime.FilterAndDedupDomains(fragments, func(d *schema.Domain) bool {
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

	for _, endpoint := range pi.Stack.Endpoint {
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
			if endpoint.GetPort().GetContainerPort() > 0 && pi.OnAdd != nil {
				pi.OnAdd(endpoint, localPort)
			}

			pi.mu.Lock()
			defer pi.mu.Unlock()

			instance.localPort = localPort
			if wasrevision == pi.revision {
				if !pi.done {
					pi.OnUpdate()
				}
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
		if pi.OnDelete != nil {
			var removed []*schema.Endpoint
			for _, key := range unused {
				removed = append(removed, pi.endpointPortFwds[key].endpoint)
			}

			pi.OnDelete(removed)
		}

		for _, key := range unused {
			delete(pi.endpointPortFwds, key)
		}
	}

	if len(domains) > 0 && pi.Env.Proto().Purpose == schema.Environment_DEVELOPMENT {
		if pi.ingressPortfwd.closer == nil {
			pi.ingressPortfwd.closer, pi.ingressPortfwd.err = runtime.For(ctx, pi.Env).ForwardIngress(ctx, []string{pi.LocalAddr}, runtime.LocalIngressPort, func(fpe runtime.ForwardedPortEvent) {
				pi.mu.Lock()
				defer pi.mu.Unlock()

				for _, port := range fpe.Added {
					// We should never receive multiple ports.
					pi.ingressPortfwd.endpoint = fpe.Endpoint
					pi.ingressPortfwd.localPort = port.LocalPort
				}

				if !pi.done {
					pi.OnUpdate()
				}
			})
		}
	} else if pi.ingressPortfwd.closer != nil {
		pi.ingressPortfwd.closer.Close()
		pi.ingressPortfwd.closer = nil
	}

	pi.OnUpdate()
}

func (pi *PortForward) Render() []byte {
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

	focus := focusServers(pi.Stack, pi.Focus)

	deploy.SortPorts(portFwds, focus)

	deploy.RenderPortsAndIngresses(true, &out, pi.LocalAddr, pi.Stack, focus, portFwds, pi.domains, nil)

	return out.Bytes()
}

func (pi *PortForward) portFwd(ctx context.Context, endpoint *schema.Endpoint, revision int, callback func(int, uint)) (io.Closer, error) {
	server := pi.Stack.GetServer(schema.PackageName(endpoint.ServerOwner))
	if server == nil {
		return nil, fnerrors.UserError(nil, "%s: missing in the stack", endpoint.ServerOwner)
	}

	return runtime.For(ctx, pi.Env).ForwardPort(ctx, server.Server, endpoint, []string{pi.LocalAddr}, func(fp runtime.ForwardedPort) {
		callback(revision, fp.LocalPort)
	})
}

func (pi *PortForward) Cleanup() error {
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

func focusServers(stack *schema.Stack, focus []schema.PackageName) []*schema.Server {
	// Must be called with lock held.

	var servers []*schema.Server
	for _, pkg := range focus {
		for _, entry := range stack.Entry {
			if entry.GetPackageName() == pkg {
				servers = append(servers, entry.Server)
				break
			}
		}
		// XXX this is a major hack, as there's no guarantee we'll see all of the
		// expected servers in the stack.
	}

	return servers
}
