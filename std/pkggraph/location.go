// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"path/filepath"
	"strings"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type Location struct {
	Module      *Module
	PackageName schema.PackageName

	relPath string
}

func (loc Location) String() string { return string(loc.PackageName) }

// Rel returns a path to the location, relative to the workspace root. If
// path segments are passed in, the combined relative path is returned.
func (loc Location) Rel(rel ...string) string {
	return filepath.Join(append([]string{loc.relPath}, rel...)...)
}

// CheckRel returns a path relative to the module. If the path attempts to
// escape the module, an error is returned.
func (loc Location) CheckRel(rel string) (string, error) {
	abs := loc.Abs(rel)
	r, err := filepath.Rel(loc.Module.absPath, abs)
	if err != nil {
		return "", fnerrors.InternalError("%s: failed to get a relative path for %q: %w", loc.PackageName, rel, err)
	}
	if strings.HasPrefix(r, "../") {
		return "", fnerrors.NewWithLocation(loc, "%s: %q attempts to leave the workspace: %s", loc.PackageName, rel, r)
	}
	return r, nil
}

// Abs returns an absolute path to the location. If path segments are passed
// in, the combined relative path is returned.
func (loc Location) Abs(rel ...string) string {
	return filepath.Join(append([]string{loc.Module.absPath, loc.relPath}, rel...)...)
}

// ErrorLocation implements the fnerrors.Location interface.
func (loc Location) ErrorLocation() string {
	return loc.relPath
}

func NewLocationForTesting(root *Module, packageName, path string) Location {
	return Location{Module: root, PackageName: schema.PackageName(packageName), relPath: path}
}
