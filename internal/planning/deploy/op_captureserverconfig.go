// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/resources"
)

func register_OpCaptureServerConfig() {
	execution.RegisterHandlerFunc(func(ctx context.Context, inv *schema.SerializedInvocation, capture *resources.OpCaptureServerConfig) (*execution.HandleResult, error) {
		c, err := execution.Get(ctx, runtime.ClusterNamespaceInjection)
		if err != nil {
			return nil, err
		}

		ctx, done := context.WithTimeout(ctx, 5*time.Minute)
		defer done()

		t := time.Now()
		if err := c.WaitUntilReady(ctx, capture.Deployable); err != nil {
			return nil, fnerrors.New("deployable never became ready in time: %w", err)
		}

		fmt.Fprintf(console.Debug(ctx), "deployable.wait-until-ready: %s: took %v\n", capture.Deployable.Id, time.Since(t))

		return &execution.HandleResult{
			Outputs: []execution.Output{
				{InstanceID: capture.ResourceInstanceId, Message: capture.ServerConfig},
			},
		}, nil
	})
}
