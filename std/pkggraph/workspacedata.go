// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"context"
	"fmt"
	"io"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/workspace"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

type EditableWorkspaceData interface {
	FormatTo(io.Writer) error

	WithModuleName(string) WorkspaceData
	WithSetDependency(...*schema.Workspace_Dependency) WorkspaceData
	WithReplacedDependencies([]*schema.Workspace_Dependency) WorkspaceData
	WithSetEnvironment(...*schema.Workspace_EnvironmentSpec) WorkspaceData
}

type WorkspaceData interface {
	cfg.Workspace

	AbsPath() string
	DefinitionFiles() []string

	EditableWorkspaceData
}

func WriteWorkspaceData(ctx context.Context, log io.Writer, vfs fnfs.ReadWriteFS, data WorkspaceData) error {
	switch len(data.DefinitionFiles()) {
	case 0:
		return fnfs.WriteWorkspaceFile(ctx, log, vfs, workspace.WorkspaceFile, func(w io.Writer) error {
			return data.FormatTo(w)
		})

	case 1:
		return fnfs.WriteWorkspaceFile(ctx, log, vfs, data.DefinitionFiles()[0], func(w io.Writer) error {
			return data.FormatTo(w)
		})

	default:
		return fnerrors.UsageError(
			fmt.Sprintf("add `--workspace_files %s` if you only want to update a single file", workspace.WorkspaceFile),
			"writing workspace data into multiple files is not supported")
	}
}
