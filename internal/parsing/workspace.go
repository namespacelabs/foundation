// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/framework/findroot"
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
	ModuleAt(context.Context, string, ...string) (pkggraph.WorkspaceData, error)
	NewModule(context.Context, string, *schema.Workspace) (pkggraph.WorkspaceData, error)
}

func FindModuleRoot(dir string) (string, error) {
	return ModuleLoader.FindModuleRoot(dir)
}

type ModuleAtArgs struct {
	SkipAPIRequirements      bool
	SkipModuleNameValidation bool
}

// Loads and validates a module at a given path.
func ModuleAt(ctx context.Context, path string, args ModuleAtArgs, workspaceFiles ...string) (pkggraph.WorkspaceData, error) {
	ws, err := ModuleLoader.ModuleAt(ctx, path, workspaceFiles...)
	if err != nil {
		return ws, err
	}

	if !args.SkipAPIRequirements {
		if err := validateAPIRequirements(ws.ModuleName(), ws.Proto().Foundation); err != nil {
			return ws, err
		}
	}

	if !args.SkipModuleNameValidation {
		if err := validateModuleName(ws.ModuleName()); err != nil {
			return ws, err
		}
	}

	return ws, nil
}

func NewModule(ctx context.Context, dir string, w *schema.Workspace) (pkggraph.WorkspaceData, error) {
	return ModuleLoader.NewModule(ctx, dir, w)
}

func RawFindModuleRoot(dir string, names ...string) (string, error) {
	return findroot.Find("workspace", dir, findroot.LookForFile(names...))
}

func validateAPIRequirements(moduleName string, w *schema.Workspace_FoundationRequirements) error {
	apiVersion := int32(versions.Builtin().APIVersion)
	if w.GetMinimumApi() > apiVersion {
		return fnerrors.NamespaceTooOld(moduleName, w.GetMinimumApi(), apiVersion)
	}

	// Check that the foundation repo dep uses an API compatible with the current CLI.
	if moduleName == foundationModule && w.GetMinimumApi() > 0 && w.GetMinimumApi() < int32(versions.Builtin().MinimumAPIVersion) {
		return fnerrors.Newf(`Unfortunately, this version of Foundation is too recent to be used with the
current repository. If you're testing out an existing repository that uses
Foundation, try fetching a newer version of the repository. If this is your
own codebase, then you'll need to either revert to a previous version of
"ns", or update your dependency versions with "ns mod get %s".

This version check will be removed in future non-alpha versions of
Foundation, which establish a stable longer term supported API surface.`, foundationModule)
	}

	return nil
}

func validateModuleName(moduleName string) error {
	if strings.ToLower(moduleName) != moduleName {
		return fnerrors.UsageError("Please run `ns mod fmt` to canonicalize the module name.", "invalid module name %q: may not contain uppercase letters", moduleName)
	}

	return nil
}
