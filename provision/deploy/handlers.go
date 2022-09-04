// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/internal/frontend/invocation"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type handlerSource struct {
	Server  provision.Server
	Package schema.PackageName
	Plan    pkggraph.PreparedProvisionPlan
}

func computeHandlers(ctx context.Context, in *stack.Stack) ([]*tool.Definition, error) {
	var handlers []*tool.Definition

	var sources []handlerSource
	for k, s := range in.ParsedServers {
		srv := in.Servers[k]
		for _, n := range s.Deps {
			sources = append(sources, handlerSource{srv, n.Package.PackageName(), n.ProvisionPlan.PreparedProvisionPlan})
		}
		sources = append(sources, handlerSource{srv, srv.PackageName(), srv.Provisioning})
	}

	for _, src := range sources {
		h, err := parseHandlers(ctx, src.Server, src.Package, src.Plan)
		if err != nil {
			return nil, err
		}
		handlers = append(handlers, h...)
	}

	sort.SliceStable(handlers, func(i, j int) bool {
		if handlers[i].TargetServer == handlers[j].TargetServer {
			return strings.Compare(handlers[i].Source.PackageName.String(), handlers[j].Source.PackageName.String()) < 0
		}

		return strings.Compare(handlers[i].TargetServer.String(), handlers[j].TargetServer.String()) < 0
	})

	return handlers, nil
}

func parseHandlers(ctx context.Context, server provision.Server, pkg schema.PackageName, pr pkggraph.PreparedProvisionPlan) ([]*tool.Definition, error) {
	source := tool.Source{
		PackageName: pkg,
		// The server in context is always implicitly declared.
		DeclaredStack: append([]schema.PackageName{server.PackageName()}, pr.DeclaredStack...),
	}

	// Determinism.
	sort.Slice(source.DeclaredStack, func(i, j int) bool {
		return strings.Compare(source.DeclaredStack[i].String(), source.DeclaredStack[j].String()) < 0
	})

	var handlers []*tool.Definition
	for _, dec := range pr.Provisioning {
		invocation, err := invocation.Make(ctx, server.SealedContext(), &server.Location, dec)
		if err != nil {
			return nil, err
		}

		handlers = append(handlers, &tool.Definition{
			TargetServer: server.PackageName(),
			Source:       source,
			Invocation:   invocation,
		})
	}

	return handlers, nil
}
