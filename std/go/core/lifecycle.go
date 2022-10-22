// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"context"
	"sync"
	"time"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
)

type CtxCloseable interface {
	Close(context.Context) error
}

type ServerResources struct {
	startupTime time.Time

	mu         sync.Mutex
	closeables []CtxCloseable
}

func (sr *ServerResources) Add(closeable CtxCloseable) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.closeables = append(sr.closeables, closeable)
}

func (sr *ServerResources) Close(ctx context.Context) error {
	sr.mu.Lock()
	closeables := sr.closeables
	sr.closeables = nil
	sr.mu.Unlock()

	var errs []error
	for k := len(closeables) - 1; k >= 0; k-- {
		if err := closeables[k].Close(ctx); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.New(errs...)
}
