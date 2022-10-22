// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Root struct {
	workspace cfg.Workspace
	editable  pkggraph.EditableWorkspaceData

	LoadedDevHost *schema.DevHost
}

var _ cfg.RootContext = &Root{}

func NewRoot(w cfg.Workspace, editable pkggraph.EditableWorkspaceData) *Root {
	return &Root{
		workspace: w,
		editable:  editable,
	}
}

func (root *Root) Abs() string                                       { return root.workspace.LoadedFrom().AbsPath }
func (root *Root) ModuleName() string                                { return root.workspace.ModuleName() }
func (root *Root) DevHost() *schema.DevHost                          { return root.LoadedDevHost }
func (root *Root) Workspace() cfg.Workspace                          { return root.workspace }
func (root *Root) EditableWorkspace() pkggraph.EditableWorkspaceData { return root.editable }
func (root *Root) ReadOnlyFS() fs.ReadDirFS                          { return fnfs.Local(root.Abs()) }
func (root *Root) ReadWriteFS() fnfs.ReadWriteFS                     { return fnfs.ReadWriteLocalFS(root.Abs()) }

func (root *Root) RelPackage(rel string) fnfs.Location {
	return fnfs.Location{
		ModuleName: root.workspace.ModuleName(),
		FS:         root.ReadWriteFS(),
		RelPath:    filepath.Clean(rel),
	}
}

// Implements fnerrors.Location.
func (root *Root) ErrorLocation() string {
	return root.Abs()
}
