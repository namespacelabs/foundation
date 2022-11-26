// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

var (
	ConfigurationInjection = Define[cfg.Configuration]("ns.configuration")
	EnvironmentInjection   = Define[*schema.Environment]("ns.schema.environment")
)

func FromContext(env cfg.Context) MakeInjectionInstance {
	return MakeInjectionInstanceFunc(func() []InjectionInstance {
		return []InjectionInstance{
			ConfigurationInjection.With(env.Configuration()),
			EnvironmentInjection.With(env.Environment()),
		}
	})
}
