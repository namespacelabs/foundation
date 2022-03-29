// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package renderwait

import (
	"context"

	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Consumer interface {
	Ch() chan ops.Event
	Wait()
}

func NewBlock(ctx context.Context, name string) Consumer {
	if tasks.ConsoleOf(tasks.SinkFrom(ctx)) == nil {
		rwb := logRenderer{
			ch:     make(chan ops.Event),
			done:   make(chan struct{}),
			logger: zerolog.Ctx(ctx),
		}
		go rwb.Loop(ctx)
		return rwb
	}

	rwb := consRenderer{
		ch:   make(chan ops.Event),
		done: make(chan struct{}),
		setSticky: func(b []byte) {
			tasks.SetStickyContent(ctx, name, b)
		},
		flushLog: console.TypedOutput(ctx, "dev", tasks.CatOutputUs),
	}
	go rwb.Loop(ctx)
	return rwb
}