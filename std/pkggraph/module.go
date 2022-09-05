// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

type Module struct {
	Workspace     *schema.Workspace
	WorkspaceData WorkspaceData

	absPath string
	// If not empty, this is an external module.
	version string
}

type MutableModule interface {
	ModuleName() string // The module that this workspace corresponds to.
	ReadWriteFS() fnfs.ReadWriteFS
}

func NewModule(w *schema.Workspace, lf *schema.Workspace_LoadedFrom, version string) *Module {
	return &Module{
		Workspace: w,
		absPath:   lf.AbsPath,
		version:   version,
	}
}

// Implements fnerrors.Location.
func (mod *Module) ErrorLocation() string {
	if mod.IsExternal() {
		return mod.Workspace.ModuleName
	}

	return mod.absPath
}

func (mod *Module) Abs() string        { return mod.absPath }
func (mod *Module) ModuleName() string { return mod.Workspace.ModuleName }

// An external module is downloaded from a remote location and stored in the cache. It always has a version.
func (mod *Module) IsExternal() bool { return mod.version != "" }

func (mod *Module) Version() string { return mod.version }
func (mod *Module) VersionedFS(rel string, observeChanges bool) compute.Computable[wscontents.Versioned] {
	return wscontents.Observe(mod.absPath, rel, observeChanges && !mod.IsExternal())
}

func (mod *Module) ReadOnlyFS() fs.FS {
	return fnfs.Local(mod.absPath)
}

func (mod *Module) ReadWriteFS() fnfs.ReadWriteFS {
	if mod.IsExternal() {
		return fnfs.Local(mod.absPath).(fnfs.ReadWriteFS) // LocalFS has a Write, which fails Writes.
	}
	return fnfs.ReadWriteLocalFS(mod.absPath)
}

func (mod *Module) MakeLocation(relPath string) Location {
	cl := filepath.Clean(relPath)
	pkg := mod.Workspace.ModuleName
	if cl != "." {
		pkg += "/" + cl
	}
	return Location{
		Module:      mod,
		PackageName: schema.PackageName(pkg),
		relPath:     cl,
	}
}

func (mod *Module) RootLocation() Location {
	return mod.MakeLocation(".")
}
