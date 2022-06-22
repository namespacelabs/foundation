// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package renderwait

import (
	"context"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
)

type Consumer interface {
	Ch() chan ops.Event
	Wait(context.Context) error
}

func NewBlock(ctx context.Context, name string) Consumer {
	if !console.IsConsoleLike(ctx) {
		rwb := logRenderer{
			ch:   make(chan ops.Event),
			done: make(chan struct{}),
		}
		go rwb.Loop(ctx)
		return rwb
	}

	rwb := consRenderer{
		ch:   make(chan ops.Event),
		done: make(chan struct{}),
		setSticky: func(b string) {
			console.SetStickyContent(ctx, name, b)
		},
		flushLog: console.TypedOutput(ctx, "dev", console.CatOutputUs),
	}
	go rwb.Loop(ctx)
	return rwb
}
