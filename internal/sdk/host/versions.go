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

func (p *ParsedVersions) SDK(version string, platform specs.Platform, binary string, makeURL func(string, specs.Platform) string) (compute.Computable[LocalSDK], error) {
	v := p.Get()

	actualVer, has := v.Versions[version]
	if !has {
		return nil, fnerrors.UserError(nil, "%s/sdk: no configuration for version %q", p.Name, version)
	}

	arts, has := v.Artifacts[actualVer]
	if !has {
		return nil, fnerrors.UserError(nil, "%s/sdk: no configuration for version %q (was %q)", p.Name, actualVer, version)
	}

	key := fmt.Sprintf("%s/%s", platform.OS, platform.Architecture)
	digest, has := arts[key]
	if !has {
		return nil, fnerrors.UserError(nil, "%s/sdk: no platform configuration for %q in %q", p.Name, key, actualVer)
	}

	ref := artifacts.Reference{
		URL: makeURL(actualVer, platform),
		Digest: schema.Digest{
			Algorithm: "sha256",
			Hex:       digest,
		},
	}

	return &PrepareSDK{Name: p.Name, Binary: binary, Version: actualVer, Ref: ref}, nil
}
