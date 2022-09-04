// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package pkggraph

import (
	"io"

	"namespacelabs.dev/foundation/schema"
)

type EditableWorkspaceData interface {
	FormatTo(io.Writer) error

	WithSetDependency(...*schema.Workspace_Dependency) WorkspaceData
	WithReplacedDependencies([]*schema.Workspace_Dependency) WorkspaceData
}

type WorkspaceData interface {
	AbsPath() string
	DefinitionFile() string
	RawData() []byte
	WorkspaceLoadedFrom() *schema.Workspace_LoadedFrom
	Parsed() *schema.Workspace

	EditableWorkspaceData
}
