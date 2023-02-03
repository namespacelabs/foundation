// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

type PreStartup interface {
	EvalStartup(context.Context, Context, StartupInputs, []ValueWithPath) (*schema.StartupPlan, error)
}

type ProvisionInputs struct {
	ServerLocation Location
}

type StartupInputs struct {
	ServerImage   string // Result of imageID.ImageRef(), not oci.ImageID to avoid cycles.
	Stack         StackEndpoints
	ServerRootAbs string
}

type StackEndpoints interface {
	EndpointsBy(schema.PackageName) []*schema.Endpoint
}

type ValueWithPath struct {
	Need  *schema.Need
	Value any
}

type PreparedProvisionPlan struct {
	ComputePlanWith []*schema.Invocation // Will generate further plan contents.
}

func (p *PreparedProvisionPlan) AppendWith(rhs PreparedProvisionPlan) {
	p.ComputePlanWith = append(p.ComputePlanWith, rhs.ComputePlanWith...)
}

type ProvisionPlan struct {
	Startup PreStartup

	// All fields on Nodes. Servers only allow specifying `Provisioning`.
	PreparedProvisionPlan

	// Server only.
	Naming *schema.Naming
}
