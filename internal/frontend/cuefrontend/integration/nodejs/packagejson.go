// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nodejs

import (
	"context"
	"encoding/json"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	packageJsonFn = "package.json"
)

type packageJson struct {
	Scripts map[string]string `json:"scripts"`
}

func readPackageJson(ctx context.Context, pl parsing.EarlyPackageLoader, loc pkggraph.Location, relPath string) (*packageJson, error) {
	fsys, err := pl.WorkspaceOf(ctx, loc.Module)
	if err != nil {
		return nil, err
	}

	jsonRaw, err := fs.ReadFile(fsys, filepath.Join(relPath, packageJsonFn))
	if err != nil {
		return nil, fnerrors.NewWithLocation(loc, "error while reading %s : %s", packageJsonFn, err)
	}

	parsedJson := &packageJson{}
	if err := json.Unmarshal(jsonRaw, &parsedJson); err != nil {
		return nil, fnerrors.NewWithLocation(loc, "error while parsing %s : %s", packageJsonFn, err)
	}

	return parsedJson, nil
}
