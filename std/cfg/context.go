// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cfg

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

func MakeUnverifiedContext(config Configuration, env *schema.Environment) Context {
	return ctx{config: config, env: env}
}

func LoadContext(parent RootContext, name string) (Context, error) {
	for _, env := range EnvsOrDefault(parent.DevHost(), parent.Workspace().Proto()) {
		if env.Name == name {
			schemaEnv := schema.SpecToEnv(env)[0]

			cfg, err := makeConfigurationCompat(parent, parent.Workspace(), ConfigurationSlice{
				Configuration:         env.Configuration,
				PlatformConfiguration: env.PlatformConfiguration,
			}, parent.DevHost(), schemaEnv)
			if err != nil {
				return nil, err
			}

			return MakeUnverifiedContext(cfg, schemaEnv), nil
		}
	}

	return nil, fnerrors.NewWithLocation(parent, "%s: no such environment", name)
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
	config Configuration
	env    *schema.Environment
}

func (e ctx) ErrorLocation() string            { return e.config.Workspace().ErrorLocation() }
func (e ctx) Workspace() Workspace             { return e.config.Workspace() }
func (e ctx) Environment() *schema.Environment { return e.env }
func (e ctx) Configuration() Configuration     { return e.config }
