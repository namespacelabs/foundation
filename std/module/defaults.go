// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package module

import (
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontendopaque"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
)

func WireDefaults() {
	parsing.ModuleLoader = cuefrontend.ModuleLoader
	parsing.MakeFrontend = func(pl parsing.EarlyPackageLoader, env *schema.Environment) parsing.Frontend {
		return cuefrontend.NewFrontend(pl, cuefrontendopaque.NewFrontend(env, pl), env)
	}
}
