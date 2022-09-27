// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"sort"
	"strings"
	"sync"

	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
)

type stackBuilder struct {
	mu      sync.Mutex
	servers map[schema.PackageName]*Server
}

func (stack *stackBuilder) claim(pkgname schema.PackageName) (*Server, bool) {
	stack.mu.Lock()
	defer stack.mu.Unlock()

	existing, has := stack.servers[pkgname]
	if has {
		return existing, true
	}

	if stack.servers == nil {
		stack.servers = map[schema.PackageName]*Server{}
	}

	ps := &Server{}
	stack.servers[pkgname] = ps
	return ps, false
}

func (stack *stackBuilder) buildStack(focus ...schema.PackageName) *Stack {
	stack.mu.Lock()
	defer stack.mu.Unlock()

	var foci uniquestrings.List
	for _, pkg := range focus {
		foci.Add(pkg.String())
	}

	s := &Stack{}

	for _, sb := range stack.servers {
		s.Servers = append(s.Servers, *sb)
	}

	sort.Slice(s.Servers, func(i, j int) bool {
		return order(foci, s.Servers[i].Server.PackageName().String(), s.Servers[j].Server.PackageName().String())
	})

	var endpoints []*schema.Endpoint
	var internal []*schema.InternalEndpoint

	for _, srv := range s.Servers {
		endpoints = append(endpoints, srv.Endpoints...)
		internal = append(internal, srv.InternalEndpoints...)
	}

	sort.Slice(endpoints, func(i, j int) bool {
		e_i := endpoints[i]
		e_j := endpoints[j]

		if e_i.ServerOwner == e_j.ServerOwner {
			return strings.Compare(e_i.AllocatedName, e_j.AllocatedName) < 0
		}
		return order(foci, e_i.ServerOwner, e_j.ServerOwner)
	})

	sort.Slice(internal, func(i, j int) bool {
		e_i := internal[i]
		e_j := internal[j]

		if e_i.ServerOwner == e_j.ServerOwner {
			return e_i.GetPort().GetContainerPort() < e_j.Port.GetContainerPort()
		}
		return order(foci, e_i.ServerOwner, e_j.ServerOwner)
	})

	s.Endpoints = endpoints
	s.InternalEndpoints = internal
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
