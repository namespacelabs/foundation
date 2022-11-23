// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	foundationModule = "namespacelabs.dev/foundation"
)

var ModuleLoader interface {
	FindModuleRoot(string) (string, error)
	ModuleAt(context.Context, string) (pkggraph.WorkspaceData, error)
}

func FindModuleRoot(dir string) (string, error) {
	return ModuleLoader.FindModuleRoot(dir)
}

type ModuleAtArgs struct {
	SkipAPIRequirements bool
}

// Loads and validates a module at a given path.
func ModuleAt(ctx context.Context, path string, args ModuleAtArgs) (pkggraph.WorkspaceData, error) {
	ws, err := ModuleLoader.ModuleAt(ctx, path)
	if err != nil {
		return ws, err
	}

	if !args.SkipAPIRequirements {
		if err := validateAPIRequirements(ws.ModuleName(), ws.Proto().Foundation); err != nil {
			return ws, err
		}
	}

	return ws, nil
}

func RawFindModuleRoot(dir string, names ...string) (string, error) {
	return findroot.Find("workspace", dir, findroot.LookForFile(names...))
}

func validateAPIRequirements(moduleName string, w *schema.Workspace_FoundationRequirements) error {
	if w.GetMinimumApi() > versions.APIVersion {
		return fnerrors.DoesNotMeetVersionRequirements(moduleName, w.GetMinimumApi(), versions.APIVersion)
	}

	// Check that the foundation repo dep uses an API compatible with the current CLI.
	if moduleName == foundationModule && w.GetMinimumApi() > 0 && w.GetMinimumApi() < versions.MinimumAPIVersion {
		return fnerrors.New(fmt.Sprintf(`Unfortunately, this version of Foundation is too recent to be used with the
current repository. If you're testing out an existing repository that uses
Foundation, try fetching a newer version of the repository. If this is your
own codebase, then you'll need to either revert to a previous version of
"ns", or update your dependency versions with "ns mod get %s".

This version check will be removed in future non-alpha versions of
Foundation, which establish a stable longer term supported API surface.`, foundationModule))
	}

	return nil
}
