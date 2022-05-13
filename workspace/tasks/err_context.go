// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"context"
	"sync"
)

var errCtx = newErrContext()

// errContext stores Attachment's buffer names that may provide additional context in case of an error.
type errContext struct {
	mu sync.Mutex
	// provides errorContext for a specific ation (keyed by ActionID)
	perAction map[string]*actionErrContext
}

type actionErrContext struct {
	mu        sync.Mutex
	buffNames []OutputName
}

func newErrContext() *errContext {
	return &errContext{
		perAction: make(map[string]*actionErrContext),
	}
}

func GetErrContext(ctx context.Context) *actionErrContext {
	actionId := Attachments(ctx).ActionID()

	errCtx.mu.Lock()
	defer errCtx.mu.Unlock()

	aec, present := errCtx.perAction[actionId]
	if !present {
		aec := &actionErrContext{
			buffNames: []OutputName{},
		}
		errCtx.perAction[actionId] = aec
	}
	return aec
}

func (aec *actionErrContext) AddLog(name OutputName) {
	aec.mu.Lock()
	defer aec.mu.Unlock()
	aec.buffNames = append(aec.buffNames, name)
}

func (aec *actionErrContext) GetBufNames() []OutputName {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	ret := make([]OutputName, len(aec.buffNames))
	// TODO prefer buffers with errors over those without errors
	copy(ret, aec.buffNames)
	return ret
}
