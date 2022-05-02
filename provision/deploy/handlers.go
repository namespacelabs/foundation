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
)

func computeHandlers(ctx context.Context, in *stack.Stack) ([]*tool.Definition, error) {
	var handlers []*tool.Definition
	for k, s := range in.ParsedServers {
		for _, n := range s.Deps {
			h, err := parseHandlers(ctx, in.Servers[k], n)
			if err != nil {
				return nil, err
			}
			handlers = append(handlers, h...)
		}
	}

	sort.SliceStable(handlers, func(i, j int) bool {
		if handlers[i].For == handlers[j].For {
			return strings.Compare(handlers[i].Source.PackageName.String(), handlers[j].Source.PackageName.String()) < 0
		}

		return strings.Compare(handlers[i].For.String(), handlers[j].For.String()) < 0
	})

	return handlers, nil
}

func parseHandlers(ctx context.Context, server provision.Server, pr *stack.ParsedNode) ([]*tool.Definition, error) {
	pkg := pr.Package
	source := tool.Source{
		PackageName: pkg.PackageName(),
		// The server in context is always implicitly declared.
		DeclaredStack: append([]schema.PackageName{server.PackageName()}, pr.ProvisionPlan.DeclaredStack...),
	}

	// Determinism.
	sort.Slice(source.DeclaredStack, func(i, j int) bool {
		return strings.Compare(source.DeclaredStack[i].String(), source.DeclaredStack[j].String()) < 0
	})

	var handlers []*tool.Definition
	for _, dec := range pr.ProvisionPlan.Provisioning {
		invocation, err := invocation.Make(ctx, server.Env(), &server.Location, dec)
		if err != nil {
			return nil, err
		}

		handlers = append(handlers, &tool.Definition{
			For:           server.PackageName(),
			ServerAbsPath: server.Location.Abs(),
			Source:        source,
			Invocation:    invocation,
		})
	}

	return handlers, nil
}
