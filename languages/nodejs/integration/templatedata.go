// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

type nodeTmplOptions struct {
	Imports   []tmplSingleImport
	Service   *tmplNodeService
	Package   tmplPackage
	Providers []tmplProvider
}

type serverTmplOptions struct {
	Imports                     []tmplSingleImport
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
	GrpcServerImportAlias string
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
	InputType  tmplImportedType
	OutputType tmplImportedType
	// nil if the provider has no dependencis.
	Deps            *tmplDeps
	PackageDepsName *string
}

type tmplDeps struct {
	Name string
	Deps []tmplDependency
}

type tmplDependency struct {
	Name              string
	Type              tmplImportedType
	Provider          tmplImportedType
	ProviderInputType tmplImportedType
	ProviderInput     tmplSerializedProto
}
type tmplSerializedProto struct {
	Base64Content string
	Comments      []string
}

type tmplImportedType struct {
	ImportAlias, Name string
}

type tmplSingleImport struct {
	Alias, Package string
}
