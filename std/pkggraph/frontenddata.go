// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

type PreProvision interface {
	EvalProvision(context.Context, planning.Context, ProvisionInputs) (ProvisionPlan, error)
}

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
	DeclaredStack   []schema.PackageName
	Sidecars        []*schema.SidecarContainer
	Inits           []*schema.SidecarContainer
	ComputePlanWith []*schema.Invocation // Will generate further plan contents.
}

func (p *PreparedProvisionPlan) AppendWith(rhs PreparedProvisionPlan) {
	p.DeclaredStack = append(p.DeclaredStack, rhs.DeclaredStack...)
	p.ComputePlanWith = append(p.ComputePlanWith, rhs.ComputePlanWith...)
	p.Sidecars = append(p.Sidecars, rhs.Sidecars...)
	p.Inits = append(p.Inits, rhs.Inits...)
}

type ProvisionPlan struct {
	Startup PreStartup

	// All fields on Nodes. Servers only allow specifying `Provisioning`.
	PreparedProvisionPlan

	// Server only.
	Naming *schema.Naming
}
