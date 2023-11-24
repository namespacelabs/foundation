// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"time"

	buildkit "github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func WaitReadiness(ctx context.Context, maxWait time.Duration, connect func(ctx context.Context) (*buildkit.Client, error)) error {
	const retryDelay = 200 * time.Millisecond

	maxRetries := maxWait.Milliseconds() / retryDelay.Milliseconds()
	for i := int64(0); i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		c, err := connect(ctx)
		if err != nil {
			return err
		}

		if _, err := c.ListWorkers(ctx); err == nil {
			return nil
		}

		time.Sleep(retryDelay)
	}

	return fnerrors.New("buildkit never became ready")
}
