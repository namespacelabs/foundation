// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package frontend

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

type PreProvision interface {
	EvalProvision(context.Context, ProvisionInputs) (ProvisionPlan, error)
}

type PreStartup interface {
	EvalStartup(context.Context, StartupInputs, []ValueWithPath) (StartupPlan, error)
}

type Location interface {
	Rel(...string) string
}

type ProvisionInputs struct {
	Env            *schema.Environment
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

type ProvisionPlan struct {
	Startup PreStartup

	// Node only.
	DeclaredStack []schema.PackageName
	Provisioning  *Invocation
	Inits         []Init

	// Server only.
	Naming *schema.Naming
}

type InvocationMount struct {
	FromWorkspace string `json:"fromWorkspace"`
}

type InvocationSnapshot struct {
	FromWorkspace string `json:"fromWorkspace"`
}

type Init struct {
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
