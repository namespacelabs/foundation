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

func WaitReadiness(ctx context.Context, connect func() (*buildkit.Client, error)) error {
	const retryDelay = 200 * time.Millisecond
	const maxRetries = 5 * 30 // 30 seconds

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
}
