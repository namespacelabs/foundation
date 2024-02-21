// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package schema

import (
	"sort"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const (
	ClearTextGrpcProtocol = "grpc"
	GrpcProtocol          = "grpcs"
	HttpProtocol          = "http"
	HttpsProtocol         = "https"
)

var (
	StaticModuleRewrites = map[string]struct {
		ModuleName string
		RelPath    string
	}{
		"library.namespace.so": {
			ModuleName: "namespacelabs.dev/foundation",
			RelPath:    "library",
		},
	}
)

func (sc *Schema) ExtsAndServices() []*Node {
	var nodes []*Node
	nodes = append(nodes, sc.GetExtension()...)
	nodes = append(nodes, sc.GetService()...)
	return nodes
}

func (n *Node) GetImportedPackages() []PackageName {
	return PackageNames(n.GetImport()...)
}

func (n *Node) ErrorLocation() string { return n.PackageName }

func (n *Node) InitializerFor(fmwk Framework) *NodeInitializer {
	if n == nil {
		return nil
	}

	for _, i := range n.Initializers {
		if i.Framework == fmwk {
			return i
		}
	}
	return nil
}

// All frameworks that the node has codegen generated for.
// Stable order.
func (n *Node) CodegeneratedFrameworks() []Framework {
	fmwksSet := map[Framework]bool{}
	if n.ServiceFramework != Framework_FRAMEWORK_UNSPECIFIED {
		fmwksSet[n.ServiceFramework] = true
	}
	for _, i := range n.Initializers {
		fmwksSet[i.Framework] = true
	}
	for p := range n.ProvidedInFrameworks() {
		fmwksSet[p] = true
	}
	fmwks := make([]Framework, 0, len(fmwksSet))
	for f := range fmwksSet {
		fmwks = append(fmwks, f)
	}
	sort.Slice(fmwks, func(i, j int) bool {
		return fmwks[i].Number() < fmwks[j].Number()
	})

	return fmwks
}

func (n *Node) ProvidedInFrameworks() map[Framework]bool {
	fmwksSet := map[Framework]bool{}
	for _, p := range n.Provides {
		for _, a := range p.AvailableIn {
			for k, v := range a.ProvidedInFrameworks() {
				fmwksSet[k] = v
			}
		}
	}
	return fmwksSet
}

func (s *Server) GetImportedPackages() []PackageName {
	return PackageNames(s.GetImport()...)
}

func (s *Server) ErrorLocation() string { return s.PackageName }

func (s *Stack) GetServer(pkg PackageName) *Stack_Entry {
	if s == nil {
		return nil
	}

	for _, e := range s.Entry {
		if pkg.Equals(e.GetServer().GetPackageName()) {
			return e
		}
	}
	return nil
}

func (s *Stack) GetServerByID(id string) *Stack_Entry {
	if s == nil {
		return nil
	}

	for _, e := range s.Entry {
		if e.GetServer().GetId() == id {
			return e
		}
	}
	return nil
}

func (s *Stack) EndpointsBy(pkgName PackageName) []*Endpoint {
	var list []*Endpoint
	for _, endpoint := range s.Endpoint {
		if pkgName.Equals(endpoint.GetServerOwner()) {
			list = append(list, endpoint)
		}
	}
	return list
}

func (se *Stack_Entry) GetPackageName() PackageName {
	return PackageName(se.GetServer().GetPackageName())
}

func (se *Stack_Entry) Extensions() []*Node {
	var services []*Node
	for _, n := range se.Node {
		if n.Kind == Node_EXTENSION {
			services = append(services, n)
		}
	}
	return services
}

func (se *Stack_Entry) Services() []*Node {
	var services []*Node
	for _, n := range se.Node {
		if n.Kind == Node_SERVICE {
			services = append(services, n)
		}
	}
	return services
}

func (p *Provides_AvailableIn) ProvidedInFrameworks() map[Framework]bool {
	fmwksSet := map[Framework]bool{}
	if p.Go != nil {
		fmwksSet[Framework_GO] = true
	}
	return fmwksSet
}

// All modules referenced in the workspace file, including the main module.
func (ws *Workspace) AllReferencedModules() []string {
	modules := []string{ws.ModuleName}

	for _, dep := range ws.Dep {
		modules = append(modules, dep.ModuleName)
	}

	for _, replace := range ws.Replace {
		modules = append(modules, replace.ModuleName)
	}

	modules = append(modules, maps.Keys(StaticModuleRewrites)...)

	slices.Sort(modules)
	return modules
}

func SpecToEnv(spec ...*Workspace_EnvironmentSpec) []*Environment {
	var envs []*Environment
	for _, env := range spec {
		envs = append(envs, &Environment{
			Name:    env.Name,
			Runtime: env.Runtime,
			Purpose: env.Purpose,
			Labels:  env.Labels,
		})
	}
	return envs
}
