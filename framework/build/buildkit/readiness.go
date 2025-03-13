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

const retryDelay = 200 * time.Millisecond

func WaitReadiness(ctx context.Context, maxWait time.Duration, connect func(ctx context.Context) (*buildkit.Client, error)) error {
	maxRetries := maxWait.Milliseconds() / retryDelay.Milliseconds()

	var i int64
	for {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		c, err := connect(ctx)
		if err != nil {
			return err
		}

		_, err = c.ListWorkers(ctx)
		if err == nil {
			return nil
		}

		if i >= maxRetries {
			return fnerrors.Newf("buildkit never became ready, last error: %w", err)
		}

		i++
		time.Sleep(retryDelay)
	}
}
