// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type Package struct {
	Location Location

	Parsed PreProvision

	// One of.
	Extension            *schema.Node
	Service              *schema.Node
	Server               *schema.Server
	ExperimentalFunction *schema.ExperimentalFunction

	// Includes auto-generated (e.g. server startup) tests.
	Tests []*schema.Test

	// Inlined or explicitly defined binaries.
	Binaries []*schema.Binary

	// Resources associated with node types.
	Provides    map[string]*protos.FileDescriptorSetAndDeps // key: `Provides.Name`
	Services    map[string]*protos.FileDescriptorSetAndDeps // key: fully qualified service name
	PackageData []*types.Resource

	// Hooks
	PrepareHooks []PrepareHook

	// Opaque-style resources.

	// Resources defined by the node.
	ResourceClasses   []*schema.ResourceClass
	ResourceProviders []*schema.ResourceProvider
	ResourceInstances []*schema.ResourceInstance

	// Resources referenced by the node.
	ProvidedResourceClasses   []*schema.ResourceClass
	RequiredResourceProviders []*schema.ResourceProvider
}

type PrepareHook struct {
	InvokeInternal string
	InvokeBinary   *schema.Invocation
}

func (pr *Package) PackageName() schema.PackageName { return pr.Location.PackageName }

func (pr *Package) Node() *schema.Node {
	if pr.Extension != nil {
		return pr.Extension
	}
	if pr.Service != nil {
		return pr.Service
	}
	return nil
}

func (pr *Package) ResourceClass(name string) *schema.ResourceClass {
	for _, rc := range pr.ResourceClasses {
		if rc.Name == name {
			return rc
		}
	}
	return nil
}

func (pr *Package) ProvidedResourceClass(pkgRef *schema.PackageRef) *schema.ResourceClass {
	for _, rc := range pr.ProvidedResourceClasses {
		if rc.PackageName == pkgRef.PackageName && rc.Name == pkgRef.Name {
			return rc
		}
	}
	return nil
}

func (pr *Package) ResourceProvider(pkgRef *schema.PackageRef) *schema.ResourceProvider {
	for _, p := range pr.ResourceProviders {
		if p.ProvidesClass.Equals(pkgRef) {
			return p
		}
	}
	return nil
}

func (pr *Package) RequiredResourceProvider(pkgRef *schema.PackageRef) *schema.ResourceProvider {
	for _, p := range pr.RequiredResourceProviders {
		if p.ProvidesClass.Equals(pkgRef) {
			return p
		}
	}
	return nil
}

func (pr *Package) ResourceInstance(name string) *schema.ResourceInstance {
	for _, r := range pr.ResourceInstances {
		if r.Name == name {
			return r
		}
	}
	return nil
}
