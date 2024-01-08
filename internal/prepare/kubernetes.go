// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

func InstantiateKube(ctx context.Context, env cfg.Context, conf *schema.DevHost_ConfigureEnvironment) (*kubernetes.Cluster, error) {
	devhost := &schema.DevHost{}
	devhost.Configure = append(devhost.Configure, conf)

	config, err := cfg.MakeConfigurationCompat(env, env.Workspace(), devhost, env.Environment())
	if err != nil {
		return nil, err
	}

	return kubernetes.ConnectToCluster(ctx, config)
}
