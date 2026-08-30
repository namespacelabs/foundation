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
	"namespacelabs.dev/foundation/internal/environment"
)

func WithSigIntCancel(ctx context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	cleanupTimeout := 2 * time.Second
	if environment.IsRunningInCI() {
		cleanupTimeout = 10 * time.Second
	}

	go func() {
		defer signal.Stop(c)

		cancelled := atomic.NewBool(false)
		for {
			select {
			case <-c:
				if cancelled.CompareAndSwap(false, true) {
					cancel()

					// Give cleanups a moment to run, and then force exit.
					go func() {
						time.Sleep(cleanupTimeout)
						os.Exit(2)
					}()
				} else if !environment.IsRunningInCI() {
					// Already cancelled, exit immediately.
					os.Exit(3)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return ctx, cancel
}
