// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc"
)

const drainTimeout = 30 * time.Second

func handleGracefulShutdown() {
	if core.EnvIs(schema.Environment_DEVELOPMENT) {
		// In development, we skip graceful shutdowns for faster iteration cycles.
		return
	}

	sigint := make(chan os.Signal, 1)

	signal.Notify(sigint, os.Interrupt)
	signal.Notify(sigint, syscall.SIGTERM)

	r2 := <-sigint

	log.Printf("got %v", r2)

	// XXX support more graceful shutdown. Although
	// https://github.com/kubernetes/kubernetes/issues/86280#issuecomment-583173036
	// "What you SHOULD do is hear the SIGTERM and start wrapping up. What
	// you should NOT do is close your listening socket. If you win the
	// race, you will receive traffic and reject it.""

	// So we start failing readiness, so we're removed from the serving set.
	// Then we wait for a bit for traffic to drain out. And then we leave.

	core.MarkShutdownStarted()

	if r2 == syscall.SIGTERM {
		if grpc.DrainFunc == nil {
			time.Sleep(drainTimeout)
		} else {
			grpc.DrainFunc()
		}

		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
