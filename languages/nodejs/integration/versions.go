// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package integration

import (
	"encoding/json"
	"io/fs"
	"sync"
)

var (
	v     versions
	vonce sync.Once
)

type versions struct {
	DevBuildDependencies map[string]string `json:"devBuildDependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
}

func builtin() *versions {
	vonce.Do(func() {
		data, err := fs.ReadFile(resources, "versions.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
	})
	return &v
}
