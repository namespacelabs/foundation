// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"context"
	"io/fs"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
)

type Package struct {
	Location Location

	PackageSources fs.FS // Filenames included will be relative to the module root, not the package.

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
	Integration *schema.PackageIntegration

	// Parsed secret definitions within the package.
	Secrets []*schema.SecretSpec

	// Parsed volume definitions within the package.
	Volumes []*schema.Volume

	// Resources associated with node types.
	Provides    map[string]*protos.FileDescriptorSetAndDeps // key: `Provides.Name`
	Services    map[string]*protos.FileDescriptorSetAndDeps // key: fully qualified service name
	PackageData []*types.Resource

	// Parsed resources
	Resources         []ResourceInstance
	ResourceClasses   []ResourceClass
	ResourceProviders []ResourceProvider

	// Hooks
	PrepareHooks []PrepareHook

	// Raw resources definitions within a package.
	ResourceClassSpecs     []*schema.ResourceClass
	ResourceProvidersSpecs []*schema.ResourceProvider
	ResourceInstanceSpecs  []*schema.ResourceInstance
}

type ResourceSpec struct {
	Source         *schema.ResourceInstance
	Class          ResourceClass
	Provider       *ResourceProvider
	ResourceInputs []ResourceInstance // Resources passed to the provider.
}

type ResourceInstance struct {
	Name *schema.PackageRef
	Spec ResourceSpec
}

type ResourceClass struct {
	Ref          *schema.PackageRef
	Source       *schema.ResourceClass
	IntentType   UserType
	InstanceType UserType
}

type ResourceProvider struct {
	Spec *schema.ResourceProvider

	Resources      []ResourceInstance
	ResourceInputs []ExpectedResourceInstance
}

type ExpectedResourceInstance struct {
	Name  *schema.PackageRef
	Class ResourceClass
}

func (rc ResourceClass) PackageName() schema.PackageName {
	return schema.PackageName(rc.Source.PackageName)
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
		if rc.Source.Name == name {
			return &rc
		}
	}
	return nil
}

func (pr *Package) LookupResourceProvider(classRef *schema.PackageRef) *ResourceProvider {
	for _, p := range pr.ResourceProviders {
		if p.Spec.ProvidesClass.Equals(classRef) {
			return &p
		}
	}

	return nil
}

func (pr *Package) LookupResourceInstance(name string) *ResourceInstance {
	for _, r := range pr.Resources {
		if r.Name.Name == name {
			return &r
		}
	}

	return nil
}

func (pr *Package) LookupSecret(name string) *schema.SecretSpec {
	for _, secret := range pr.Secrets {
		if secret.Name == name {
			return secret
		}
	}

	return nil
}

func (rp ResourceProvider) LookupExpected(name *schema.PackageRef) *ExpectedResourceInstance {
	for _, x := range rp.ResourceInputs {
		if x.Name.Equals(name) {
			return &x
		}
	}

	return nil
}

func LookupResourceClass(ctx context.Context, pl PackageLoader, ref *schema.PackageRef) (*ResourceClass, error) {
	pkg, err := pl.LoadByName(ctx, ref.AsPackageName())
	if err != nil {
		return nil, err
	}

	rc := pkg.LookupResourceClass(ref.Name)
	if rc == nil {
		return nil, fnerrors.BadInputError("resource class %q not found in package %q", ref.Name, ref.PackageName)
	}

	return rc, nil
}
