// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package versions

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

var (
	//go:embed versions.json
	lib embed.FS
)

type InternalVersions struct {
	// APIVersion represents the overall version of Namespaces's semantics, which
	// are built into Namespace itself (i.e. is not versioned as part of the
	// foundation repository). Whenever new non-backwards compatible semantics are
	// added to Namespace, this number must be bumped.
	APIVersion int `json:"api_version"`
	// MinimumAPIVersion represents the minimum requested version that this version
	// of foundation supports. If a module requests, e.g. a minimum version of 28,
	// which is below the version here specified, then Namespace will fail with a
	// error that says our version of Namespace is too recent. This is used during
	// development when maintaining backwards compatibility is too expensive.
	MinimumAPIVersion int `json:"minimum_api_version"`
	CacheVersion      int `json:"cache_version"`
}

var (
	internalVersions InternalVersions
	loadOnce         sync.Once
)

func Builtin() InternalVersions {
	loadOnce.Do(func() {
		v, err := LoadAt(lib, "versions.json")
		if err != nil {
			panic(err.Error())
		}
		internalVersions = v
	})

	return internalVersions
}

func LoadAt(fsys fs.FS, path string) (InternalVersions, error) {
	versionData, err := fs.ReadFile(fsys, path)
	if err != nil {
		return InternalVersions{}, fnerrors.InternalError("failed to load version data: %w", err)
	}

	var v InternalVersions
	if err := json.Unmarshal(versionData, &v); err != nil {
		return InternalVersions{}, fnerrors.InternalError("failed to unmarshal version data: %w", err)
	}

	return v, nil
}

func LoadAtOrDefaults(fsys fs.FS, path string) (InternalVersions, error) {
	v, err := LoadAt(fsys, path)
	if err != nil {
		if os.IsNotExist(err) {
			return LastNonJSONVersion(), nil
		}
		return InternalVersions{}, err
	}
	return v, nil
}

func LastNonJSONVersion() InternalVersions {
	return InternalVersions{
		APIVersion:        44,
		MinimumAPIVersion: 40,
		CacheVersion:      1,
	}
}

const IntroducedGrpcTranscodeNode = 35

// Embedded into provisioning tools.
const ToolAPIVersion = 4

const ToolsIntroducedCompression = 3
const ToolsIntroducedInlineInvocation = 4
