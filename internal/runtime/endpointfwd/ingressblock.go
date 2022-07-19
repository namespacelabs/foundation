// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package endpointfwd

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

type PortForward struct {
	LocalAddr string
	Env       *schema.Environment
	Debug     io.Writer
	Warnings  io.Writer

	ForwardPort    func(*schema.Server, int32, []string, runtime.SinglePortForwardedFunc) (io.Closer, error)
	ForwardIngress func([]string, int, runtime.PortForwardedFunc) (io.Closer, error)

	OnAdd    func(*schema.Endpoint, uint)
	OnDelete func([]*schema.Endpoint)
	OnUpdate func()

	mu            sync.Mutex
	stack         *schema.Stack
	focus         []*schema.Server
	done          bool
	revision      int
	endpointState map[string]*endpointState
	ingressState  localPortFwd
	localPorts    map[string]*localPortFwd
	domains       []*runtime.FilteredDomain
	fragments     []*schema.IngressFragment
}

type endpointState struct {
	endpoint *schema.Endpoint
	revision int
	port     *localPortFwd
}

type localPortFwd struct {
	users     []*endpointState
	closer    io.Closer
	err       error
	revision  int
	localPort uint
}

func (pi *PortForward) Update(stack *schema.Stack, focus []schema.PackageName, fragments []*schema.IngressFragment) {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	pi.stack = stack
	pi.focus = focusServers(stack, focus)

	pi.domains = runtime.FilterAndDedupDomains(fragments, func(d *schema.Domain) bool {
		return d.GetManaged() != schema.Domain_MANAGED_UNKNOWN
	})
	pi.fragments = fragments

	pi.revision++
	fmt.Fprintf(pi.Debug, "portfwd: revision: %d\n", pi.revision)

	newState := map[string]*endpointState{}
	for _, endpoint := range stack.Endpoint {
		key := fmt.Sprintf("%s/%s/%s", endpoint.ServerOwner, endpoint.EndpointOwner, endpoint.ServiceName)
		newState[key] = &endpointState{endpoint: endpoint, revision: pi.revision}
	}

	if pi.OnDelete != nil {
		var removed []*schema.Endpoint
		for key, fwd := range pi.endpointState {
			if newState[key] == nil {
				removed = append(removed, fwd.endpoint)
			}
		}

		pi.OnDelete(removed)
	}

	pi.endpointState = newState

	type portReq struct {
		ServerOwner   string
		ContainerPort int32
		Users         []*endpointState
	}

	portRequirements := map[string]*portReq{} // server/port --> endpoint state
	for _, fwd := range newState {
		portKey := fmt.Sprintf("%s/%d", fwd.endpoint.ServerOwner, fwd.endpoint.Port.ContainerPort)
		existing, has := portRequirements[portKey]
		if !has {
			existing = &portReq{ServerOwner: fwd.endpoint.ServerOwner, ContainerPort: fwd.endpoint.Port.ContainerPort}
			portRequirements[portKey] = existing
		}

		existing.Users = append(existing.Users, fwd)
	}

	if pi.localPorts == nil {
		pi.localPorts = map[string]*localPortFwd{}
	}

	for key, reqs := range portRequirements {
		existing := pi.localPorts[key]
		if existing != nil {
			existing.users = reqs.Users
			existing.revision = pi.revision
			continue
		}

		instance := &localPortFwd{users: reqs.Users, revision: pi.revision}

		reqs := reqs // Close reqs
		closer, err := pi.portFwd(reqs.ServerOwner, reqs.ContainerPort, pi.revision, func(wasrevision int, localPort uint) {
			pi.mu.Lock()
			defer pi.mu.Unlock()

			fmt.Fprintf(pi.Debug, "portfwd: event: revisions: %d/%d localPort: %d done: %v\n",
				wasrevision, pi.revision, localPort, pi.done)

			if wasrevision != pi.revision {
				return
			}

			instance.localPort = localPort
			if !pi.done {
				if pi.OnAdd != nil {
					for _, user := range instance.users {
						pi.OnAdd(user.endpoint, localPort)
					}
				}

				pi.OnUpdate()
			}
		})

		instance.closer = closer
		instance.err = err
		if err != nil {
			fmt.Fprintf(pi.Warnings, "%s/%d: failed to forward port: %v\n", reqs.ServerOwner, reqs.ContainerPort, err)
		}

		pi.localPorts[key] = instance
	}

	for key, reqs := range portRequirements {
		for _, user := range reqs.Users {
			user.port = pi.localPorts[key]
		}
	}

	for key := range pi.localPorts {
		instance := pi.localPorts[key]
		if instance.revision < pi.revision {
			if instance.closer != nil {
				instance.closer.Close()
			}
			delete(pi.localPorts, key)
		}
	}

	if len(pi.domains) > 0 && pi.Env.GetPurpose() == schema.Environment_DEVELOPMENT {
		if pi.ingressState.closer == nil {
			pi.ingressState.closer, pi.ingressState.err = pi.ForwardIngress([]string{pi.LocalAddr}, runtime.LocalIngressPort, func(fpe runtime.ForwardedPortEvent) {
				pi.mu.Lock()
				defer pi.mu.Unlock()

				for _, port := range fpe.Added {
					// We should never receive multiple ports.
					pi.ingressState.users = []*endpointState{
						{endpoint: fpe.Endpoint, port: &pi.ingressState},
					}
					pi.ingressState.localPort = port.LocalPort
				}

				if !pi.done {
					pi.OnUpdate()
				}
			})
			if pi.ingressState.err != nil {
				fmt.Fprintf(pi.Warnings, "failed to forward ingress: %v\n", pi.ingressState.err)
			}
		}
	} else if pi.ingressState.closer != nil {
		pi.ingressState.closer.Close()
		pi.ingressState.closer = nil
	}

	pi.OnUpdate()
}

func (pi *PortForward) Render(style colors.Style) string {
	var portFwds []*deploy.PortFwd
	for _, fwd := range pi.endpointState {
		portFwds = append(portFwds, &deploy.PortFwd{
			Endpoint:  fwd.endpoint,
			LocalPort: fwd.port.localPort,
		})
	}

	if len(pi.ingressState.users) > 0 {
		portFwds = append(portFwds, &deploy.PortFwd{
			Endpoint:  pi.ingressState.users[0].endpoint,
			LocalPort: pi.ingressState.localPort,
		})
	}

	deploy.SortPorts(portFwds, pi.focus)

	var out bytes.Buffer
	r := deploy.RenderPortsAndIngresses(pi.LocalAddr, pi.stack, pi.focus, portFwds, pi.domains, pi.fragments)
	deploy.RenderText(&out, style, r, true, pi.LocalAddr)
	return out.String()
}

func (pi *PortForward) portFwd(serverOwner string, containerPort int32, revision int, callback func(int, uint)) (io.Closer, error) {
	server := pi.stack.GetServer(schema.PackageName(serverOwner))
	if server == nil {
		return nil, fnerrors.UserError(nil, "%s: missing in the stack", serverOwner)
	}

	return pi.ForwardPort(server.Server, containerPort, []string{pi.LocalAddr}, func(fp runtime.ForwardedPort) {
		callback(revision, fp.LocalPort)
	})
}

func (pi *PortForward) Cleanup() error {
	pi.mu.Lock()
	defer pi.mu.Unlock()

	pi.done = true

	var errs []error
	for _, fwd := range pi.localPorts {
		if fwd.closer != nil {
			if err := fwd.closer.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if pi.ingressState.closer != nil {
		if err := pi.ingressState.closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
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
