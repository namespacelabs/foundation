// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildsetup

import (
	"namespacelabs.dev/foundation/internal/build/binary"
	golangintegration "namespacelabs.dev/foundation/internal/integrations/golang"
	"namespacelabs.dev/foundation/std/cfg"
)

func ConfigureGoBuilder(env cfg.Context) {
	if golangintegration.GoBuilderKind.Get(env.Configuration()) != golangintegration.GoBuilderMaybeBazel {
		return
	}

	builder := golangintegration.MaybeBazelBuilder(env.Workspace().LoadedFrom().AbsPath)
	binary.BuildGo = builder.GoBuilder
	golangintegration.ConfigureBuilder(builder)
}
