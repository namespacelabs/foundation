// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"runtime/debug"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func Version() (string, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "", fnerrors.InternalError("no builtin debug information?")
	}

	for _, d := range bi.Deps {
		if d.Path == "github.com/moby/buildkit" {
			return d.Version, nil
		}
	}

	return "", fnerrors.InternalError("buildkit: vendored version is empty")
}
