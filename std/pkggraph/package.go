// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

	// Integration that has been applied to this package. May be nil.
	// Shouldn't be used outside of workspace.SealPackage.
	Integration *schema.Integration

	// Resources associated with node types.
	Provides    map[string]*protos.FileDescriptorSetAndDeps // key: `Provides.Name`
	Services    map[string]*protos.FileDescriptorSetAndDeps // key: fully qualified service name
	PackageData []*types.Resource

	// Parsed resources
	Resources       []Resource
	ResourceClasses []ResourceClass

	// Hooks
	PrepareHooks []PrepareHook

	// Raw resources definitions within a package.
	ResourceClassSpecs    []*schema.ResourceClass
	ResourceProviders     []*schema.ResourceProvider
	ResourceInstanceSpecs []*schema.ResourceInstance
}

type Resource struct {
	Spec            *schema.ResourceInstance
	Class           ResourceClass
	ProviderPackage *Package
	Provider        *schema.ResourceProvider
}

type ResourceClass struct {
	Spec         *schema.ResourceClass
	IntentType   UserType
	InstanceType UserType
}

func (rc ResourceClass) PackageName() schema.PackageName {
	return schema.PackageName(rc.Spec.PackageName)
}

type UserType struct {
	Descriptor protoreflect.MessageDescriptor
	Sources    *protos.FileDescriptorSetAndDeps
	Registry   *protoregistry.Files
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

func (pr *Package) LookupBinary(name string) (*schema.Binary, error) {
	for _, bin := range pr.Binaries {
		if bin.Name == name {
			return bin, nil
		}
	}

	if name == "" && len(pr.Binaries) == 1 {
		return pr.Binaries[0], nil
	}

	return nil, fnerrors.UserError(pr.Location, "no such binary %q", name)
}

func (pr *Package) LookupResourceClass(name string) *ResourceClass {
	for _, rc := range pr.ResourceClasses {
		if rc.Spec.Name == name {
			return &rc
		}
	}
	return nil
}

func (pr *Package) LookupResourceProvider(classRef *schema.PackageRef) *schema.ResourceProvider {
	for _, p := range pr.ResourceProviders {
		if p.ProvidesClass.Equals(classRef) {
			return p
		}
	}

	return nil
}

func (pr *Package) LookupResourceInstance(name string) *schema.ResourceInstance {
	for _, r := range pr.ResourceInstanceSpecs {
		if r.Name == name {
			return r
		}
	}
	return nil
}
