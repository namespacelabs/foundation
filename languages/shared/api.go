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
	Service   *ServiceData
	Providers []ProviderData
}

type ServiceData struct {
	Deps []DependencyData
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
}

type DependencyData struct {
	Name             string
	Provider         ProviderData
	ProviderLocation workspace.Location
	ProviderInput    SerializedProto
}

type SerializedProto struct {
	Base64Content string
	Comments      []string
}
