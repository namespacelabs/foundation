// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type RootContext interface {
	fnerrors.Location
	DevHost() *schema.DevHost
	Workspace() Workspace
}

type Context interface {
	fnerrors.Location
	Workspace() Workspace
	Configuration() Configuration
	Environment() *schema.Environment
}

func MakeUnverifiedContext(config Configuration, ws Workspace, env *schema.Environment, errorLocation string) Context {
	return ctx{config: config, errorLocation: errorLocation, workspace: ws, env: env}
}

func LoadContext(parent RootContext, name string) (Context, error) {
	for _, env := range EnvsOrDefault(parent.DevHost(), parent.Workspace().Proto()) {
		if env.Name == name {
			schemaEnv := schema.SpecToEnv(env)[0]

			cfg, err := makeConfigurationCompat(parent, ConfigurationSlice{
				Configuration:         env.Configuration,
				PlatformConfiguration: env.PlatformConfiguration,
			}, parent.DevHost(), schemaEnv)
			if err != nil {
				return nil, err
			}

			return MakeUnverifiedContext(cfg, parent.Workspace(), schemaEnv, parent.ErrorLocation()), nil
		}
	}

	return nil, fnerrors.UserError(parent, "%s: no such environment", name)
}

func EnvsOrDefault(devHost *schema.DevHost, workspace *schema.Workspace) []*schema.Workspace_EnvironmentSpec {
	base := workspace.EnvSpec
	if base == nil {
		base = []*schema.Workspace_EnvironmentSpec{
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
		}
	}

	return append(slices.Clone(devHost.LocalEnv), base...)
}

type ctx struct {
	config        Configuration
	errorLocation string
	workspace     Workspace
	env           *schema.Environment
}

func (e ctx) ErrorLocation() string            { return e.errorLocation }
func (e ctx) Workspace() Workspace             { return e.workspace }
func (e ctx) Environment() *schema.Environment { return e.env }
func (e ctx) Configuration() Configuration     { return e.config }
