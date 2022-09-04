// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/planning"
)

type WorkspaceEnvironment interface {
	planning.Context
	Packages
}

type MutableWorkspaceEnvironment interface {
	WorkspaceEnvironment

	ModuleName() string // The module that this workspace corresponds to.
	OutputFS() fnfs.ReadWriteFS
}
