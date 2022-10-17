// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tool

import (
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/schema"
)

type Source struct {
	PackageName   schema.PackageName
	DeclaredStack []schema.PackageName // Handlers can only configure servers that were configured by the source.
}

type Definition struct {
	TargetServer schema.PackageName
	Source       Source // Where the invocation was declared.
	Invocation   *invocation.Invocation
}

func (s Source) Contains(pkg schema.PackageName) bool {
	for _, d := range s.DeclaredStack {
		if d == pkg {
			return true
		}
	}
	return false
}
