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
	EvalStartup(context.Context, ops.Environment, StartupInputs, []ValueWithPath) (StartupPlan, error)
}

type Location interface {
	Rel(...string) string
}

type ProvisionInputs struct {
	Workspace      *schema.Workspace
	ServerLocation Location
}

type StartupInputs struct {
	ServerImage string // Result of imageID.ImageRef(), not oci.ImageID to avoid cycles.
	Stack       *schema.Stack
	Server      *schema.Server
}

type ValueWithPath struct {
	Need  *schema.Need
	Value interface{}
}

type ProvisionStack struct {
	DeclaredStack []schema.PackageName
}

type ProvisionPlan struct {
	Startup PreStartup

	// Node only.
	ProvisionStack
	Provisioning *Invocation
	Sidecars     []Container
	Inits        []Container

	// Server only.
	Naming *schema.Naming
}

type InvocationMount struct {
	FromWorkspace string `json:"fromWorkspace"`
}

type InvocationSnapshot struct {
	FromWorkspace string `json:"fromWorkspace"`
	Optional      bool   `json:"optional"`
	RequireFile   bool   `json:"requireFile"`
}

type Container struct {
	Binary string   `json:"binary"`
	Args   []string `json:"args"`
}

type Invocation struct {
	Binary       string                        `json:"binary"`
	Args         []string                      `json:"args"`
	Mounts       map[string]InvocationMount    `json:"mount"`
	WorkingDir   string                        `json:"workingDir"`
	Snapshots    map[string]InvocationSnapshot `json:"snapshot"`
	NoCache      bool                          `json:"noCache"`
	RequiresKeys bool                          `json:"requiresKeys"`
}

type StartupPlan struct {
	Args []string          `json:"args"`
	Env  map[string]string `json:"env"`
}
