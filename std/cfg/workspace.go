// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cfg

import (
	"io/fs"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

var (
	DefaultWorkspaceEnvironments = []*schema.Workspace_EnvironmentSpec{
		{
			Name:    "dev",
			Runtime: "kubernetes",
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
)

type Workspace interface {
	fnerrors.Location

	Proto() *schema.Workspace
	ModuleName() string
	ReadOnlyFS(rel ...string) fs.FS
	LoadedFrom() *schema.Workspace_LoadedFrom
}

type workspace struct {
	proto      *schema.Workspace
	loadedFrom *schema.Workspace_LoadedFrom
}

func MakeSyntheticWorkspace(proto *schema.Workspace, lf *schema.Workspace_LoadedFrom) Workspace {
	return workspace{proto, lf}
}

func (w workspace) ErrorLocation() string                    { return w.proto.ModuleName }
func (w workspace) Proto() *schema.Workspace                 { return w.proto }
func (w workspace) ModuleName() string                       { return w.proto.ModuleName }
func (w workspace) ReadOnlyFS(rel ...string) fs.FS           { return nil }
func (w workspace) LoadedFrom() *schema.Workspace_LoadedFrom { return w.loadedFrom }
