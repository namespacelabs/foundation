// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
)

type Root struct {
	workspace  *schema.Workspace
	loadedFrom *schema.Workspace_LoadedFrom
	editable   pkggraph.EditableWorkspaceData

	LoadedDevHost *schema.DevHost
}

var _ planning.RootContext = &Root{}

func NewRoot(w *schema.Workspace, lf *schema.Workspace_LoadedFrom, editable pkggraph.EditableWorkspaceData) *Root {
	return &Root{
		workspace:  w,
		loadedFrom: lf,
		editable:   editable,
	}
}

func (root *Root) Abs() string                                       { return root.loadedFrom.AbsPath }
func (root *Root) ModuleName() string                                { return root.workspace.ModuleName }
func (root *Root) DevHost() *schema.DevHost                          { return root.LoadedDevHost }
func (root *Root) Workspace() *schema.Workspace                      { return root.workspace }
func (root *Root) WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom { return root.loadedFrom }
func (root *Root) EditableWorkspace() pkggraph.EditableWorkspaceData { return root.editable }
func (root *Root) ReadOnlyFS() fs.ReadDirFS                          { return fnfs.Local(root.Abs()) }
func (root *Root) ReadWriteFS() fnfs.ReadWriteFS                     { return fnfs.ReadWriteLocalFS(root.Abs()) }

func (root *Root) RelPackage(rel string) fnfs.Location {
	return fnfs.Location{
		ModuleName: root.workspace.ModuleName,
		FS:         root.ReadWriteFS(),
		RelPath:    filepath.Clean(rel),
	}
}

// Implements fnerrors.Location.
func (root *Root) ErrorLocation() string {
	return root.Abs()
}
