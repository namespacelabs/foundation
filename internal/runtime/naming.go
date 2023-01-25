// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

var (
	NamingNoTLS = false // Set to true in CI.

	WorkInProgressUseShortAlias = false
)

func ComputeNaming(ctx context.Context, moduleName string, env cfg.Context, planner Planner, source *schema.Naming) (*schema.ComputedNaming, error) {
	naming, err := planner.Ingress().ComputeNaming(env.Environment(), source)
	if err != nil {
		return nil, err
	}

	naming.MainModuleName = moduleName
	naming.UseShortAlias = naming.GetUseShortAlias() || WorkInProgressUseShortAlias

	fmt.Fprintf(console.Debug(ctx), "computed naming: %+v\n", naming)

	return naming, nil
}
