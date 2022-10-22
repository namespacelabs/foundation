// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cfg

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type Workspace interface {
	fnerrors.Location

	Proto() *schema.Workspace
	ModuleName() string

	LoadedFrom() *schema.Workspace_LoadedFrom
}

type workspace struct {
	proto      *schema.Workspace
	loadedFrom *schema.Workspace_LoadedFrom
}

func MakeWorkspace(proto *schema.Workspace, lf *schema.Workspace_LoadedFrom) Workspace {
	return workspace{proto, lf}
}

func (w workspace) ErrorLocation() string                    { return w.proto.ModuleName }
func (w workspace) Proto() *schema.Workspace                 { return w.proto }
func (w workspace) ModuleName() string                       { return w.proto.ModuleName }
func (w workspace) LoadedFrom() *schema.Workspace_LoadedFrom { return w.loadedFrom }
