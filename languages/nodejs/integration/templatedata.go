// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import "namespacelabs.dev/foundation/languages/nodejs/imports"

type nodeTmplOptions struct {
	Imports   []imports.SingleImport
	Service   *tmplNodeService
	Package   tmplPackage
	Providers []tmplProvider
}

type serverTmplOptions struct {
	Imports                     []imports.SingleImport
	Services                    []tmplServerService
	ImportedInitializersAliases []string
}

type nodeImplTmplOptions struct {
	ServiceServerName, ServiceName, ServiceFileName string
}

type tmplServerService struct {
	Type    tmplImportedType
	HasDeps bool
}

type tmplNodeService struct {
}

type tmplPackage struct {
	Name        string
	Initializer *tmplInitializer
	// nil if the package has no dependencies.
	Deps *tmplDeps
	// De-duped import aliases of dependencies.
	ImportedInitializers []string
}

type tmplInitializer struct {
	InitializeBefore []string
	InitializeAfter  []string
	PackageDepsName  *string
}

type tmplProvider struct {
	Name       string
	InputType  *tmplImportedType
	OutputType tmplImportedType
	// nil if the provider has no dependencis.
	Deps            *tmplDeps
	PackageDepsName *string
	// If parametrized, the provider function has a type parameter and
	// takes an additional argument - constructor of that type.
	// Used to implement the gRPC extension where the provided type is
	// a usage-specific gRPC client instance.
	IsParameterized bool
}

type tmplDeps struct {
	Name string
	Deps []tmplDependency
}

type tmplDependency struct {
	Name              string
	Type              tmplImportedType
	Provider          tmplImportedType
	ProviderInputType *tmplImportedType
	ProviderInput     tmplSerializedProto
	// If the provider is parameterized, this contains a factory function for creating instances of the provided value.
	ProviderOutputFactoryType *tmplImportedType
}
type tmplSerializedProto struct {
	Base64Content string
	Comments      []string
}

type tmplImportedType struct {
	ImportAlias, Name string
	Parameters        []tmplImportedType
}
