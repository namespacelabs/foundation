// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"context"

	"namespacelabs.dev/foundation/workspace/tasks"
)

func SetIdleLabel(ctx context.Context, label string) {
	unwrapped := UnwrapSink(tasks.SinkFrom(ctx))
	if t, ok := unwrapped.(consoleLike); ok {
		t.SetIdleLabel(label)
	}
}

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
	SetIdleLabel(string)
	SetStickyContent(string, []byte)
	EnterInputMode(context.Context, ...string) func()
}
