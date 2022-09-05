// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type UnboundContext interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
	DevHost() *schema.DevHost
}

type Context interface {
	UnboundContext

	Configuration() Configuration
	Environment() *schema.Environment
}

func MakeUnverifiedContext(ws *schema.Workspace, lf *schema.Workspace_LoadedFrom, devhost *schema.DevHost, env *schema.Environment, errorLocation string) Context {
	return ctx{errorLocation: errorLocation, workspace: ws, loadedFrom: lf, devHost: devhost, env: env}
}

func LoadContext(parent UnboundContext, name string) (Context, error) {
	for _, env := range EnvsOrDefault(parent.DevHost(), parent.Workspace()) {
		if env.Name == name {
			return MakeUnverifiedContext(parent.Workspace(), parent.WorkspaceLoadedFrom(), parent.DevHost(), env, parent.ErrorLocation()), nil
		}
	}

	return nil, fnerrors.UserError(nil, "no such environment: %s", name)
}

func EnvsOrDefault(devHost *schema.DevHost, workspace *schema.Workspace) []*schema.Environment {
	baseEnvs := slices.Clone(devHost.LocalEnv)

	if workspace.Env != nil {
		return append(baseEnvs, workspace.Env...)
	}

	return append(baseEnvs, []*schema.Environment{
		{
			Name:    "dev",
			Runtime: "kubernetes", // XXX
			Purpose: schema.Environment_DEVELOPMENT,
		},
		{
			Name:    "staging",
			Runtime: "kubernetes",
			Purpose: schema.Environment_PRODUCTION,
		},
		{
			Name:    "prod",
			Runtime: "kubernetes",
			Purpose: schema.Environment_PRODUCTION,
		},
	}...)
}

type ctx struct {
	errorLocation string
	workspace     *schema.Workspace
	loadedFrom    *schema.Workspace_LoadedFrom
	devHost       *schema.DevHost
	env           *schema.Environment
}

func (e ctx) ErrorLocation() string                             { return e.errorLocation }
func (e ctx) Workspace() *schema.Workspace                      { return e.workspace }
func (e ctx) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return e.loadedFrom }
func (e ctx) DevHost() *schema.DevHost                          { return e.devHost }
func (e ctx) Environment() *schema.Environment                  { return e.env }
func (e ctx) Configuration() Configuration                      { return MakeConfigurationCompat(e) }
