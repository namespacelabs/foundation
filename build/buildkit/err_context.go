// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"sync"

	"namespacelabs.dev/foundation/workspace/tasks"
)

// errContext stores Attachment's buffer names that may provide additional context in case of an error.
type errContext struct {
	mu sync.Mutex
	// provides errorContext for a specific ation (keyed by ActionID)
	perAction map[string]*actionErrContext
}

type actionErrContext struct {
	buffNames   []tasks.OutputName
	buffWithErr map[tasks.OutputName]bool // A buffer is associated with a Vertex known to fail.
}

func newErrContext() *errContext {
	return &errContext{
		perAction: make(map[string]*actionErrContext),
	}
}

func (ec *errContext) addLog(ctx context.Context, name tasks.OutputName) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	actionId := tasks.Attachments(ctx).ActionID()

	aec, present := ec.perAction[actionId]
	if !present {
		aec := &actionErrContext{
			buffNames:   []tasks.OutputName{name},
			buffWithErr: make(map[tasks.OutputName]bool),
		}
		ec.perAction[actionId] = aec
	} else {
		aec.buffNames = append(aec.buffNames, name)
	}
}

// markLogHasErr indicates that a given log buffer may store relevant information about an user visible error.
// markLogHasErr should be called after a given name has been added.
func (ec *errContext) markLogHasErr(ctx context.Context, name tasks.OutputName) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	actionId := tasks.Attachments(ctx).ActionID()

	aec, present := ec.perAction[actionId]
	if !present {
		return // call addLog first
	}
	aec.buffWithErr[name] = true
}

func (ec *errContext) getBufNames(ctx context.Context) []tasks.OutputName {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	actionId := tasks.Attachments(ctx).ActionID()
	aec, present := ec.perAction[actionId]
	if !present {
		return nil // This action hasn't produced any logs.
	}
	out := make([]tasks.OutputName, 0, len(aec.buffNames))
	for name := range aec.buffWithErr {
		out = append(out, name)
	}
	if len(out) > 0 {
		// We are happy to present those buffers only.
		return out
	}
	// If we don't have any buffers with errors, we can present all buffers.
	return aec.buffNames
}
