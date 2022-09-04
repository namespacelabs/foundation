// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

type WorkspaceEnvironment interface {
	planning.Context
	Packages
}

type MutableWorkspaceEnvironment interface {
	WorkspaceEnvironment
	pkggraph.MutableModule
}
