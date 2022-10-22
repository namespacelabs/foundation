// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"io"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

type EditableWorkspaceData interface {
	FormatTo(io.Writer) error

	WithSetDependency(...*schema.Workspace_Dependency) WorkspaceData
	WithReplacedDependencies([]*schema.Workspace_Dependency) WorkspaceData
}

type WorkspaceData interface {
	cfg.Workspace

	AbsPath() string
	DefinitionFile() string
	RawData() []byte

	EditableWorkspaceData
}
