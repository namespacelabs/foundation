// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
