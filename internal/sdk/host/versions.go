// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package host

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type VersionConfiguration struct {
	Versions  map[string]string       `json:"versions"`
	Artifacts map[string]ArtifactList `json:"artifacts"`
}

type ArtifactList map[string]string // platform --> digest

type ParsedVersions struct {
	Name string
	FS   fs.FS

	v    VersionConfiguration
	once sync.Once
}

func (p *ParsedVersions) Get() *VersionConfiguration {
	p.once.Do(func() {
		data, err := fs.ReadFile(p.FS, "versions.json")
		if err != nil {
			panic(err)
		}
		if err := json.Unmarshal(data, &p.v); err != nil {
			panic(err)
		}
	})

	return &p.v
}

func (p *ParsedVersions) SDK(version string, platform specs.Platform, makeURL func(string, specs.Platform) (string, string)) (compute.Computable[LocalSDK], error) {
	v := p.Get()

	actualVer, has := v.Versions[version]
	if !has {
		return nil, fnerrors.New("%s/sdk: no configuration for version %q", p.Name, version)
	}

	arts, has := v.Artifacts[actualVer]
	if !has {
		return nil, fnerrors.New("%s/sdk: no configuration for version %q (was %q)", p.Name, actualVer, version)
	}

	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	digest, has := arts[key]
	if !has {
		return nil, fnerrors.New("%s/sdk: no platform configuration for %q in %q", p.Name, key, actualVer)
	}

	url, binary := makeURL(actualVer, platform)

	ref := artifacts.Reference{
		URL: url,
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       digest,
		},
	}

	return &PrepareSDK{Name: p.Name, Platform: platform, Binary: binary, Version: actualVer, Ref: ref}, nil
}
