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

	// Opaque-style resources.
	ResourceClasses []*schema.ResourceClass

	// Hooks
	PrepareHooks []PrepareHook
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
