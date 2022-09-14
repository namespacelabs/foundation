// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"encoding/json"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	packageJsonFn = "package.json"
)

type packageJson struct {
	Scripts map[string]string `json:"scripts"`
}

func readPackageJson(loc pkggraph.Location) (*packageJson, error) {
	jsonRaw, err := fs.ReadFile(loc.Module.ReadOnlyFS(), filepath.Join(loc.Rel(), packageJsonFn))
	if err != nil {
		return nil, fnerrors.UserError(loc, "error while reading %s : %s", packageJsonFn, err)
	}

	parsedJson := &packageJson{}
	if err := json.Unmarshal(jsonRaw, &parsedJson); err != nil {
		return nil, fnerrors.UserError(loc, "error while parsing %s : %s", packageJsonFn, err)
	}

	return parsedJson, nil
}
