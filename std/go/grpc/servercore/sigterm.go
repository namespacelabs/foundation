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

func handleGracefulShutdown(ctx context.Context, finishShutdown func()) {
	sigint := make(chan os.Signal, 1)

	signal.Notify(sigint, os.Interrupt)
	signal.Notify(sigint, syscall.SIGTERM)

	select {
	case r2 := <-sigint:
		core.ZLog.Info().Str("signal", r2.String()).Msg("got signal")

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

		if nsgrpc.DrainFunc != nil {
			nsgrpc.DrainFunc()
		}

		delta := time.Since(t)
		if delta < readinessPropagationDelay {
			dur := readinessPropagationDelay - delta
			core.ZLog.Info().Dur("duration", dur).Msg("sleeping before final shutdown")
			time.Sleep(dur)
		}

		finishShutdown()
	case <-ctx.Done():
		return
	}
}
