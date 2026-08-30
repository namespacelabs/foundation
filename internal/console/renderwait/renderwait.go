// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package renderwait

import (
	"context"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/schema/orchestration"
)

type Consumer interface {
	Ch() chan *orchestration.Event
	Wait(context.Context) error
}

func NewBlock(ctx context.Context, name string) Consumer {
	if !console.IsConsoleLike(ctx) {
		rwb := logRenderer{
			ch:   make(chan *orchestration.Event),
			done: make(chan struct{}),
		}
		go rwb.Loop(ctx)
		return rwb
	}

	rwb := consRenderer{
		ch:   make(chan *orchestration.Event),
		done: make(chan struct{}),
		setSticky: func(b string) {
			console.SetStickyContent(ctx, name, b)
		},
		flushLog: console.TypedOutput(ctx, "dev", console.CatOutputUs),
	}
	go rwb.Loop(ctx)
	return rwb
}
