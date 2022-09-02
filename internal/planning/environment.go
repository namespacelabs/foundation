// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package planning

import (
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

// Environment represents an execution environment: it puts together a root
// workspace, a workspace configuration (devhost) and then finally the
// schema-level environment we're running for.
type Environment interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
	DevHost() *schema.DevHost
	Proto() *schema.Environment // Will be nil if not in a build or deployment phase.
}
