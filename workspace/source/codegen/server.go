// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package codegen

import (
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func ForServer(pkg *workspace.Package, available []*schema.Node) ([]*schema.Definition, error) {
	defs, err := languages.IntegrationFor(pkg.Server.Framework).GenerateServer(pkg, available)
	if err != nil {
		return nil, err
	}

	return defs, nil
}

func ForServerAndDeps(server provision.Server) ([]*schema.Definition, error) {
	var allDefs []*schema.Definition
	for _, dep := range server.Deps() {
		// We only update co-located nodes.
		if dep.Location.Module.ModuleName() == server.Location.Module.ModuleName() {
			defs, err := ProtosForNode(dep)
			if err != nil {
				return nil, err
			}
			allDefs = append(allDefs, defs...)

			defs, err = ForNodeForLanguage(dep, server.StackEntry().Node)
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
