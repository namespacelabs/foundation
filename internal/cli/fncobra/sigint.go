// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"os"
	"os/signal"
	"time"

	"go.uber.org/atomic"
)

func WithSigIntCancel(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	cancelled := atomic.NewBool(false)

	go func() {
		select {
		case <-c:
			if cancelled.CAS(false, true) {
				cancel()

				// Give cleanups a moment to run, and then force exit.
				time.Sleep(2 * time.Second)
				os.Exit(2)
			} else {
				// Already cancelled, exit immediately.
				os.Exit(3)
			}
		case <-ctx.Done():
		}
	}()
	return ctx, func() {
		signal.Stop(c)
		cancel()
	}
}
