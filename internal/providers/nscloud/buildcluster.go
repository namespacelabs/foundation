// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"

	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func EnsureBuildCluster(ctx context.Context, _ api.API) (*buildkit.Overrides, error) {
	cfg, err := api.EnsureBuildCluster(ctx, "amd64")
	if err != nil {
		return nil, err
	}

	return &buildkit.Overrides{
		HostedBuildCluster: &buildkit.HostedBuildCluster{
			ClusterId: cfg.InstanceId,
			Endpoint:  cfg.Endpoint,
		},
	}, nil
}
