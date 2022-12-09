// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/runtime"
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
func (mod *Module) Snapshot(rel string) compute.Computable[wscontents.Versioned] {
	return wscontents.Observe(mod.absPath, rel, !mod.IsExternal())
}

func (mod *Module) ChangeTrigger(rel string) compute.Computable[compute.Versioned] {
	return wscontents.ChangeTrigger(mod.absPath, rel)
}

func (mod *Module) ReadOnlyFS(rel ...string) fs.FS {
	return fnfs.Local(mod.absPath, rel...)
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

func (mod *Module) VCS(ctx context.Context) (*runtime.BuildVCS, error) {
	t := time.Now()

	status, err := git.FetchStatus(ctx, mod.absPath)
	if err != nil {
		return nil, err
	}

	if status.Revision == "" {
		fmt.Fprintf(console.Debug(ctx), "module.vcs: %s: none detected\n", mod.ModuleName())
		return nil, nil
	}

	fmt.Fprintf(console.Debug(ctx), "module.vcs: %s: revision %s (uncommited: %v) commit_time %v (took %v)\n",
		mod.ModuleName(), status.Revision, status.Uncommitted, status.CommitTime, time.Since(t))

	return &runtime.BuildVCS{
		Revision:    status.Revision,
		CommitTime:  status.CommitTime.Format(time.RFC3339),
		Uncommitted: status.Uncommitted,
	}, nil
}
