// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tool

import (
	"io/fs"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

type Source struct {
	PackageName   schema.PackageName
	DeclaredStack []schema.PackageName // Handlers can only configure servers that were configured by the source.
}

type Definition struct {
	For           schema.PackageName
	ServerAbsPath string
	Source        Source
	Invocation    *Invocation
}

type Invocation struct {
	ImageName  string
	Image      compute.Computable[oci.Image]
	Command    []string
	Args       []string
	Mounts     []*rtypes.LocalMapping
	Snapshots  []Snapshot
	WorkingDir string
	NoCache    bool
}

type Snapshot struct {
	Name     string
	Contents fs.FS
}

func (s Source) Contains(pkg schema.PackageName) bool {
	for _, d := range s.DeclaredStack {
		if d == pkg {
			return true
		}
	}
	return false
}
