// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/dirs"
)

type Root struct {
	Workspace *schema.Workspace
	DevHost   *schema.DevHost

	absPath string
}

func NewRoot(absPath string, w *schema.Workspace) *Root {
	return &Root{
		Workspace: w,
		absPath:   absPath,
	}
}

func (root *Root) Abs() string { return root.absPath }

func (root *Root) FS() fnfs.LocalFS {
	return fnfs.ReadWriteLocalFS(root.absPath)
}

func (root *Root) RelPackage(rel string) fnfs.Location {
	return fnfs.Location{
		ModuleName: root.Workspace.ModuleName,
		FS:         root.FS(),
		RelPath:    filepath.Clean(rel),
	}
}

// Implements fnerrors.Location.
func (root *Root) ErrorLocation() string {
	return root.absPath
}

func (root *Root) CacheSymLink() string {
	return filepath.Join(root.absPath, dirs.WorkspaceCacheSymLink)
}
