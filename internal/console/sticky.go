// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package console

import (
	"context"

	"namespacelabs.dev/foundation/std/tasks"
)

func SetStickyContent(ctx context.Context, name string, content string) {
	SetStickyContentOnSink(tasks.SinkFrom(ctx), name, content)
}

func SetStickyContentOnSink(sink tasks.ActionSink, name string, content string) {
	unwrapped := UnwrapSink(sink)
	if t, ok := unwrapped.(consoleLike); ok {
		t.SetStickyContent(name, []byte(content))
	}
}

func EnterInputMode(ctx context.Context, params ...string) func() {
	unwrapped := UnwrapSink(tasks.SinkFrom(ctx))
	if t, ok := unwrapped.(consoleLike); ok {
		return t.EnterInputMode(ctx, params...)
	}
	return func() {}
}

func IsConsoleLike(ctx context.Context) bool {
	unwrapped := UnwrapSink(tasks.SinkFrom(ctx))
	if _, ok := unwrapped.(consoleLike); ok {
		return ok
	}
	return false
}

type consoleLike interface {
	SetStickyContent(string, []byte)
	EnterInputMode(context.Context, ...string) func()
}
