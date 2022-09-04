// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

// Context represents an execution environment: it puts together a root
// workspace, a workspace configuration (devhost) and then finally the
// schema-level environment we're running for.
type Context interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
	DevHost() *schema.DevHost
	Environment() *schema.Environment
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
