// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package build

import (
	"context"
	"io/fs"
	"strings"
	"time"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var (
	FixedPoint       = time.Unix(1, 1)
	platformOverride = []specs.Platform{}
)

type Spec interface {
	BuildImage(context.Context, pkggraph.SealedContext, Configuration) (compute.Computable[oci.Image], error)
	PlatformIndependent() bool
}

type Plan struct {
	SourcePackage schema.PackageName
	SourceLabel   string
	BuildKind     storage.Build_Kind
	Spec          Spec
	Workspace     Workspace
	Platforms     []specs.Platform

	// The caller has given us a hint or where the built image will be uploaded
	// to, in case the builder implementation can use this information for
	// optimization purposes. This may be null, and an implementation can always
	// elect to ignore it.
	PublishName compute.Computable[oci.AllocatedRepository]
}

func (p Plan) GetSourceLabel() string { return p.SourceLabel }
func (p Plan) GetSourcePackageRef() *schema.PackageRef {
	return schema.MakePackageSingleRef(p.SourcePackage)
}

type Workspace interface {
	ModuleName() string
	Abs() string
	ReadOnlyFS(rel ...string) fs.FS

	// ChangeTrigger returns an observable which will get a new value whenever a
	// path under `rel` is modified, and the filter function doesn't reject.
	// Excludes is a list of excluded files, in buildkit format.
	ChangeTrigger(rel string, excludes []string) compute.Computable[any]
}

type BuildTarget interface {
	SourcePackage() schema.PackageName
	SourceLabel() string

	TargetPlatform() *specs.Platform
	// See Plan.PublishName.
	PublishName() compute.Computable[oci.AllocatedRepository]
}

type Configuration interface {
	BuildTarget

	// If the builder has the ability to produce with buildkit, that's
	// preferred. A reason to do this is for instance when we want to merge
	// multiple images together, and want to defer the merge to buildkit.
	PrefersBuildkit() bool
	Workspace() Workspace
}

type BuildPlatformsVar struct{}

func (BuildPlatformsVar) String() string {
	var p []string
	for _, plat := range platformOverride {
		p = append(p, platform.FormatPlatform(plat))
	}
	return strings.Join(p, ",")
}

func (BuildPlatformsVar) Set(s string) error {
	platformParts := strings.Split(s, ",")

	var ps []specs.Platform
	for _, p := range platformParts {
		parsed, err := platform.ParsePlatform(p)
		if err != nil {
			return err
		}
		ps = append(ps, parsed)
	}

	platformOverride = ps
	return nil
}

func (BuildPlatformsVar) Type() string {
	return ""
}

func PlatformsOrOverrides(platforms []specs.Platform) []specs.Platform {
	if len(platformOverride) > 0 {
		return platformOverride
	}
	return platforms
}
