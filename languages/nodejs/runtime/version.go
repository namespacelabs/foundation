// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"embed"
	"encoding/json"
	"io/fs"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

var (
	//go:embed package.json
	resources   embed.FS
	packageJson runtimePackageJson
	vonce       sync.Once
)

// Extracting the actual runtime version directly from package.json
type runtimePackageJson struct {
	Version string `json:"version"`
}

func RuntimeVersion() string {
	vonce.Do(func() {
		data, err := fs.ReadFile(resources, "package.json")
		if err != nil {
			fnerrors.Panic(err)
		}
		if err := json.Unmarshal(data, &packageJson); err != nil {
			fnerrors.Panic(err)
		}
	})
	return packageJson.Version
}
