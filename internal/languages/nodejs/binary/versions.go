// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"embed"
	"encoding/json"
	"io/fs"
	"sync"
)

var (
	//go:embed versions.json
	versionsFile embed.FS
	v            versionsJson
	vonce        sync.Once
)

type versionsJson struct {
	Pnpm string `json:"pnpm"`
}

func versions() *versionsJson {
	vonce.Do(func() {
		data, err := fs.ReadFile(versionsFile, "versions.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &v); err != nil {
			panic(err)
		}
	})
	return &v
}
