// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package endpointfwd

import (
	"fmt"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	deploystorage "namespacelabs.dev/foundation/internal/planning/deploy/storage"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

type PortForward struct {
	LocalAddr string
	Env       *schema.Environment
	Debug     io.Writer
	Warnings  io.Writer

	ForwardPort    func(runtime.Deployable, int32, []string, runtime.SinglePortForwardedFunc) (io.Closer, error)
	ForwardIngress func([]string, int, runtime.PortForwardedFunc) (io.Closer, error)

	OnAdd    func(*schema.Endpoint, uint)
	OnDelete func([]*schema.Endpoint)
	OnUpdate func(*storage.NetworkPlan)

	mu            sync.Mutex
	stack         *schema.Stack
	focus         []schema.PackageName
	done          bool
	revision      int
	endpointState map[string]*endpointState
	ingressState  localPortFwd
	localPorts    map[string]*localPortFwd
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
	pi.focus = focus

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

				pi.OnUpdate(pi.toNetworkPlan())
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

	hasDomains := false
	for _, frag := range fragments {
		if frag.Domain.GetManaged() != schema.Domain_MANAGED_UNKNOWN &&
			frag.Domain.GetFqdn() != "" {
			hasDomains = true
			break
		}
	}

	if hasDomains && pi.Env.GetPurpose() == schema.Environment_DEVELOPMENT {
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
					pi.OnUpdate(pi.toNetworkPlan())
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

	pi.OnUpdate(pi.toNetworkPlan())
}

func (pi *PortForward) toNetworkPlan() *storage.NetworkPlan {
	// No update yet.
	if pi.revision == 0 {
		return nil
	}

	var portFwds []*deploystorage.PortFwd
	for _, fwd := range pi.endpointState {
		portFwds = append(portFwds, &deploystorage.PortFwd{
			Endpoint:  fwd.endpoint,
			LocalPort: uint32(fwd.port.localPort),
		})
	}

	if len(pi.ingressState.users) > 0 {
		portFwds = append(portFwds, &deploystorage.PortFwd{
			Endpoint:  pi.ingressState.users[0].endpoint,
			LocalPort: uint32(pi.ingressState.localPort),
		})
	}

	return deploystorage.ToStorageNetworkPlan(pi.LocalAddr, pi.stack, pi.focus, portFwds, pi.fragments)
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
