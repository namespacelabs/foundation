// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package module

import (
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/std/cfg"
)

func NewPackageLoader(env cfg.Context) *parsing.PackageLoader {
	return parsing.NewPackageLoader(env)
}
