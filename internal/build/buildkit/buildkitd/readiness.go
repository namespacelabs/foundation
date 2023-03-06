// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkitd

import (
	"context"
	"time"

	buildkit "github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func WaitReadiness(ctx context.Context, connect func() (*buildkit.Client, error)) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		const retryDelay = 200 * time.Millisecond
		const maxRetries = 5 * 60 // 60 seconds

		c, err := connect()
		if err != nil {
			return err
		}

		for i := 0; i < maxRetries; i++ {
			if _, err := c.ListWorkers(ctx); err == nil {
				return nil
			}

			time.Sleep(retryDelay)
		}

		return fnerrors.New("buildkit never became ready")
	})
}
