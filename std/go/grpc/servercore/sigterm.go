// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"namespacelabs.dev/foundation/std/go/core"
	nsgrpc "namespacelabs.dev/foundation/std/go/grpc"
)

// How long do we expect it would take Kubernetes to notice that we're no longer ready.
const readinessPropagationDelay = 15 * time.Second

// runShutdownPhases waits for readiness propagation, then runs the lameduck
// phase followed by the drain phase.
func runShutdownPhases(waitForReadinessPropagation func(), beginLameduck func() map[string]func(), drainFunc func(), drainFuncs map[string]func()) {
	waitForReadinessPropagation()

	// Lameduck phase: signal clients that we're going away (e.g. send
	// HTTP/2 GOAWAY) before we start blocking on in-flight work. This
	// gives clients a head start to migrate to other backends while the
	// drain phase below waits for the requests we already have to
	// complete.
	for name, f := range beginLameduck() {
		core.ZLog.Info().Str("name", name).Msg("running lameduck func")
		f()
	}

	if drainFunc != nil {
		drainFunc()
	}

	for _, f := range drainFuncs {
		f()
	}
}

func handleGracefulShutdown(ctx context.Context, finishShutdown func()) {
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)

	select {
	case r2 := <-sigint:
		core.ZLog.Info().Str("signal", r2.String()).Msg("got signal")

		// Allow a repeated signal to terminate us ungracefully.
		signal.Stop(sigint)

		// XXX support more graceful shutdown. Although
		// https://github.com/kubernetes/kubernetes/issues/86280#issuecomment-583173036
		// "What you SHOULD do is hear the SIGTERM and start wrapping up. What
		// you should NOT do is close your listening socket. If you win the
		// race, you will receive traffic and reject it.""
		//
		// So we start failing readiness, so we're removed from the serving set.
		// Then we wait for a bit for traffic to drain out. And then we leave.

		t := time.Now()
		core.MarkShutdownStarted()

		runShutdownPhases(func() {
			delta := time.Since(t)
			if delta < readinessPropagationDelay {
				dur := readinessPropagationDelay - delta
				core.ZLog.Info().Dur("duration", dur).Msg("waiting for readiness propagation")
				time.Sleep(dur)
			}
		}, nsgrpc.BeginLameduck, nsgrpc.DrainFunc, nsgrpc.DrainFuncsByName)

		finishShutdown()
	case <-ctx.Done():
		return
	}
}
