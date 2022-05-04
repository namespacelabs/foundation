// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package frontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/schema"
)

type PreProvision interface {
	EvalProvision(context.Context, ops.Environment, ProvisionInputs) (ProvisionPlan, error)
}

type PreStartup interface {
	EvalStartup(context.Context, ops.Environment, StartupInputs, []ValueWithPath) (*schema.StartupPlan, error)
}

type Location interface {
	Rel(...string) string
}

type ProvisionInputs struct {
	Workspace      *schema.Workspace
	ServerLocation Location
}

type StartupInputs struct {
	ServerImage   string // Result of imageID.ImageRef(), not oci.ImageID to avoid cycles.
	Stack         *schema.Stack
	Server        *schema.Server
	ServerRootAbs string
}

type ValueWithPath struct {
	Need  *schema.Need
	Value interface{}
}

type ProvisionStack struct {
	DeclaredStack []schema.PackageName
}

type PreparedProvisionPlan struct {
	ProvisionStack
	Provisioning []*schema.Invocation
	Sidecars     []*schema.SidecarContainer
	Inits        []*schema.SidecarContainer
}

func (p *PreparedProvisionPlan) AppendWith(rhs PreparedProvisionPlan) {
	p.DeclaredStack = append(p.DeclaredStack, rhs.DeclaredStack...)
	p.Provisioning = append(p.Provisioning, rhs.Provisioning...)
	p.Sidecars = append(p.Sidecars, rhs.Sidecars...)
	p.Inits = append(p.Inits, rhs.Inits...)
}

type ProvisionPlan struct {
	Startup PreStartup

	// Node only.
	PreparedProvisionPlan

	// Server only.
	Naming *schema.Naming
}
