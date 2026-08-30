// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ServerData struct {
	Services             []EmbeddedServiceData
	ImportedInitializers []pkggraph.Location
}

type EmbeddedServiceData struct {
	Location pkggraph.Location
	HasDeps  bool
}

type NodeData struct {
	Kind                 schema.Node_Kind
	PackageName          string
	Deps                 []DependencyData
	Providers            []ProviderData
	ImportedInitializers []pkggraph.Location
	Initializer          *PackageInitializerData
}

type PackageInitializerData struct {
	// List of packages that need to be initialized before this package. Enforced at runtime.
	InitializeBefore []string
	InitializeAfter  []string
}

type ProtoTypeKind int32

const (
	ProtoMessage ProtoTypeKind = iota
	ProtoService
)

type ProtoTypeData struct {
	Name           string
	SourceFileName string
	Location       pkggraph.Location
	// Distinguishing between message and service types because they need to be imported from different files in node.js
	Kind ProtoTypeKind
}

type ProviderData struct {
	Name         string
	Location     pkggraph.Location
	InputType    ProtoTypeData
	ProviderType ProviderTypeData
	ScopedDeps   []DependencyData
}

// Only one of these two fields is set.
type ProviderTypeData struct {
	// Regular case: the user specific the type of the provider in `availableIn`.
	ParsedType *schema.Provides_AvailableIn
	// std/grpc extension: the provider type `<service-name>Client` is generated at runtime.
	// Only can happen within DependencyData.
	Type *ProtoTypeData
	// If true, the provider can return different types dependning on the usage context.\
	// Used to implement gRPC client injection.
	IsParameterized bool
}

type DependencyData struct {
	Name              string
	ProviderName      string
	ProviderInputType ProtoTypeData
	ProviderType      ProviderTypeData
	ProviderLocation  pkggraph.Location
	ProviderInput     SerializedProto
}

type SerializedProto struct {
	Content  []byte
	Comments []string
}
