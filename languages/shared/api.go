// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shared

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type ServerData struct {
	Services []EmbeddedServiceData
}

type EmbeddedServiceData struct {
	Location workspace.Location
}

type NodeData struct {
	Kind        schema.Node_Kind
	PackageName string
	Deps        []DependencyData
	Providers   []ProviderData
}

type TypeData struct {
	Name           string
	SourceFileName string
	PackageName    schema.PackageName
}

type ProviderData struct {
	Name         string
	InputType    TypeData
	ProviderType *schema.Provides_AvailableIn
	ScopedDeps   []DependencyData
}

type DependencyData struct {
	Name              string
	ProviderName      string
	ProviderInputType TypeData
	ProviderType      *schema.Provides_AvailableIn
	ProviderLocation  workspace.Location
	ProviderInput     SerializedProto
}

type SerializedProto struct {
	Content  []byte
	Comments []string
}
