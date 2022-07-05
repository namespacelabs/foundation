// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package stack

import (
	"context"
	"sort"
	"strings"
	"sync"

	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
)

type stackBuilder struct {
	mu        sync.Mutex
	servers   []serverBuilder
	endpoints []*schema.Endpoint
	internal  []*schema.InternalEndpoint
	known     map[schema.PackageName]struct{} // TODO consider removing this and fully relying on `servers`
}

type serverBuilder struct {
	srv    provision.Server
	parsed *ParsedServer
}

func newStackBuilder() *stackBuilder {
	return &stackBuilder{
		known: make(map[schema.PackageName]struct{}),
	}
}

func (stack *stackBuilder) Add(srv provision.Server) *ParsedServer {
	ps := &ParsedServer{}

	stack.mu.Lock()
	defer stack.mu.Unlock()

	stack.known[srv.PackageName()] = struct{}{}
	stack.servers = append(stack.servers, serverBuilder{srv, ps})
	return ps
}

func (stack *stackBuilder) checkAdd(ctx context.Context, env provision.ServerEnv, pkgname schema.PackageName) (*provision.Server, *ParsedServer, error) {
	stack.mu.Lock()

	if _, ok := stack.known[pkgname]; ok {
		stack.mu.Unlock()
		return nil, nil, nil
	}

	stack.known[pkgname] = struct{}{}
	stack.mu.Unlock()

	childT, err := provision.RequireServer(ctx, env, pkgname)
	if err != nil {
		return nil, nil, err
	}

	ps := stack.Add(childT)

	return &childT, ps, nil
}

func (stack *stackBuilder) AddEndpoints(endpoints []*schema.Endpoint, internal []*schema.InternalEndpoint) {
	stack.mu.Lock()
	defer stack.mu.Unlock()
	stack.endpoints = append(stack.endpoints, endpoints...)
	stack.internal = append(stack.internal, internal...)
}

func (stack *stackBuilder) Seal(focus ...schema.PackageName) *Stack {
	stack.mu.Lock()
	defer stack.mu.Unlock()

	var foci uniquestrings.List
	for _, pkg := range focus {
		foci.Add(pkg.String())
	}

	sort.Slice(stack.servers, func(i, j int) bool {
		return order(foci, stack.servers[i].srv.PackageName().String(), stack.servers[j].srv.PackageName().String())
	})

	sort.Slice(stack.endpoints, func(i, j int) bool {
		e_i := stack.endpoints[i]
		e_j := stack.endpoints[j]

		if e_i.ServerOwner == e_j.ServerOwner {
			return strings.Compare(e_i.AllocatedName, e_j.AllocatedName) < 0
		}
		return order(foci, e_i.ServerOwner, e_j.ServerOwner)
	})

	sort.Slice(stack.internal, func(i, j int) bool {
		e_i := stack.internal[i]
		e_j := stack.internal[j]

		if e_i.ServerOwner == e_j.ServerOwner {
			return e_i.GetPort().GetContainerPort() < e_j.Port.GetContainerPort()
		}
		return order(foci, e_i.ServerOwner, e_j.ServerOwner)
	})

	s := &Stack{
		Endpoints:         stack.endpoints,
		InternalEndpoints: stack.internal,
	}

	for _, sb := range stack.servers {
		s.Servers = append(s.Servers, sb.srv)
		s.ParsedServers = append(s.ParsedServers, sb.parsed)
	}

	return s
}

func order(foci uniquestrings.List, a, b string) bool {
	if foci.Has(a) {
		if !foci.Has(b) {
			return true
		}
	} else if foci.Has(b) {
		return false
	}

	return strings.Compare(a, b) < 0
}
