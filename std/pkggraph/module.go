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
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/runtime"
)

type Module struct {
	Workspace *schema.Workspace

	loadedFrom *schema.Workspace_LoadedFrom
	// If not empty, this is an external module.
	version string
}

type MutableModule interface {
	ModuleName() string // The module that this workspace corresponds to.
	ReadWriteFS() fnfs.ReadWriteFS
}

func NewModule(w *schema.Workspace, lf *schema.Workspace_LoadedFrom, version string) *Module {
	return &Module{
		Workspace:  w,
		loadedFrom: lf,
		version:    version,
	}
}

// Implements fnerrors.Location.
func (mod *Module) ErrorLocation() string {
	if mod.IsExternal() {
		return mod.Workspace.ModuleName
	}

	return mod.loadedFrom.AbsPath
}

func (mod *Module) Abs() string                                                         { return mod.loadedFrom.AbsPath }
func (mod *Module) DefinitionFiles() []string                                           { return mod.loadedFrom.DefinitionFiles }
func (mod *Module) ModuleName() string                                                  { return mod.Workspace.ModuleName }
func (mod *Module) Version() string                                                     { return mod.version }
func (mod *Module) ChangeTrigger(rel string, excludes []string) compute.Computable[any] { return nil }

// An external module is downloaded from a remote location and stored in the cache. It always has a version.
func (mod *Module) IsExternal() bool { return mod.version != "" }

func (mod *Module) ReadOnlyFS(rel ...string) fs.FS {
	return fnfs.Local(mod.Abs(), rel...)
}

func (mod *Module) ReadWriteFS() fnfs.ReadWriteFS {
	if mod.IsExternal() {
		return fnfs.Local(mod.Abs()).(fnfs.ReadWriteFS) // LocalFS has a Write, which fails Writes.
	}
	return fnfs.ReadWriteLocalFS(mod.Abs())
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

	status, err := git.FetchStatus(ctx, mod.Abs())
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
