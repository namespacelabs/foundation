// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package genpackage

import (
	"context"

	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func ForServer(pkg *pkggraph.Package, available []*schema.Node) ([]*schema.SerializedInvocation, error) {
	defs, err := integrations.IntegrationFor(pkg.Server.Framework).GenerateServer(pkg, available)
	if err != nil {
		return nil, err
	}

	return defs, nil
}

func ForServerAndDeps(ctx context.Context, server planning.Server) ([]*schema.SerializedInvocation, error) {
	var allDefs []*schema.SerializedInvocation
	for _, dep := range server.Deps() {
		// We only update co-located nodes.
		if dep.Location.Module.ModuleName() == server.Location.Module.ModuleName() {
			defs, err := ProtosForNode(dep)
			if err != nil {
				return nil, err
			}
			allDefs = append(allDefs, defs...)

			defs, err = ForNodeForLanguage(ctx, dep, server.StackEntry().Node)
			if err != nil {
				return nil, err
			}

			allDefs = append(allDefs, defs...)
		}
	}

	defs, err := ForServer(server.Package, server.StackEntry().Node)
	if err != nil {
		return nil, err
	}

	// XXX order reproducibility.
	allDefs = append(allDefs, defs...)
	return allDefs, nil
}
