// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type RootContext interface {
	fnerrors.Location
	DevHost() *schema.DevHost
	Workspace() *schema.Workspace
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
}

type Context interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
	Configuration() Configuration
	Environment() *schema.Environment
}

func MakeUnverifiedContext(config Configuration, ws *schema.Workspace, lf *schema.Workspace_LoadedFrom, env *schema.Environment, errorLocation string) Context {
	return ctx{config: config, errorLocation: errorLocation, workspace: ws, loadedFrom: lf, env: env}
}

func LoadContext(parent RootContext, name string) (Context, error) {
	for _, env := range EnvsOrDefault(parent.DevHost(), parent.Workspace()) {
		if env.Name == name {
			cfg := MakeConfigurationCompat(parent.Workspace(), parent.DevHost(), env)
			return MakeUnverifiedContext(cfg, parent.Workspace(), parent.WorkspaceLoadedFrom(), env, parent.ErrorLocation()), nil
		}
	}

	return nil, fnerrors.UserError(nil, "no such environment: %s", name)
}

func EnvsOrDefault(devHost *schema.DevHost, workspace *schema.Workspace) []*schema.Environment {
	if workspace.EnvSpec != nil {
		return specToEnv(workspace.EnvSpec...)
	}

	return append(specToEnv(devHost.LocalEnv...), []*schema.Environment{
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

func specToEnv(spec ...*schema.Workspace_EnvironmentSpec) []*schema.Environment {
	var envs []*schema.Environment
	for _, env := range spec {
		envs = append(envs, &schema.Environment{
			Name:    env.Name,
			Runtime: env.Runtime,
			Purpose: env.Purpose,
			Labels:  env.Labels,
		})
	}
	return envs
}

type ctx struct {
	config        Configuration
	errorLocation string
	workspace     *schema.Workspace
	loadedFrom    *schema.Workspace_LoadedFrom
	env           *schema.Environment
}

func (e ctx) ErrorLocation() string                             { return e.errorLocation }
func (e ctx) Workspace() *schema.Workspace                      { return e.workspace }
func (e ctx) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return e.loadedFrom }
func (e ctx) Environment() *schema.Environment                  { return e.env }
func (e ctx) Configuration() Configuration                      { return e.config }
