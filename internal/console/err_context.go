// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package console

import (
	"context"
	"sync"

	"namespacelabs.dev/foundation/std/tasks"
)

var errCtx = newErrContext()

// errContext stores Attachment's buffer names that may provide additional context in case of an error.
type errContext struct {
	mu sync.Mutex
	// provides errorContext for a specific action (keyed by ActionID).
	perAction map[tasks.ActionID]*actionErrContext
}

type actionErrContext struct {
	mu        sync.Mutex
	buffNames []tasks.OutputName
}

func newErrContext() *errContext {
	return &errContext{
		perAction: make(map[tasks.ActionID]*actionErrContext),
	}
}

// WithLogs adds additional context to the error message, but only if a given message
// hasn't been output in the most recent log lines.
func WithLogs(ctx context.Context, err error) error {
	// Disable logs processing until the formatter below is updated to deal
	// with structured logs.
	return err
}

func (err *actionErrContext) AddLog(name tasks.OutputName) {
	err.mu.Lock()
	defer err.mu.Unlock()
	err.buffNames = append(err.buffNames, name)
}

func GetErrContext(ctx context.Context) *actionErrContext {
	actionId := tasks.Attachments(ctx).ActionID()

	errCtx.mu.Lock()
	defer errCtx.mu.Unlock()

	aec, present := errCtx.perAction[actionId]
	if !present {
		aec = &actionErrContext{
			buffNames: []tasks.OutputName{},
		}
		errCtx.perAction[actionId] = aec
	}
	return aec
}
