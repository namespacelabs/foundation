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
}

type ProviderData struct {
	Name         string
	InputType    *schema.TypeDef
	ProviderType *schema.Provides_AvailableIn
}
