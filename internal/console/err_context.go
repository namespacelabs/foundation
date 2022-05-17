// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package console

import (
	"context"
	"io"
	"sync"

	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
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
	buffNames []tasks.OutputName
}

func newErrContext() *errContext {
	return &errContext{
		perAction: make(map[string]*actionErrContext),
	}
}

// WithLogs adds additional context to the error message, but only if a given message
// hasn't been output in the most recent log lines.
func WithLogs(ctx context.Context, err error) error {
	sink := tasks.SinkFrom(ctx)
	if sink == nil {
		return err
	}
	attachments := tasks.Attachments(ctx)
	if consoleSink, ok := sink.(*consolesink.ConsoleSink); ok {
		// Only skip the error message
		if consoleSink.RecentInputSourcesContain(attachments.ActionID()) {
			return err
		}
	}

	bufNames := GetErrContext(ctx).getBufNames()
	for i := range bufNames {
		err = fnerrors.WithLogs(
			err,
			func() io.Reader {
				return attachments.ReaderByOutputName(bufNames[len(bufNames)-i-1])
			})
		// TODO: allow multi buffer as contexts. As for now we use the last buffer as a heuristic.
		break
	}

	return err
}

func (aec *actionErrContext) AddLog(name tasks.OutputName) {
	aec.mu.Lock()
	defer aec.mu.Unlock()
	aec.buffNames = append(aec.buffNames, name)
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

func (aec *actionErrContext) getBufNames() []tasks.OutputName {
	aec.mu.Lock()
	defer aec.mu.Unlock()

	ret := make([]tasks.OutputName, len(aec.buffNames))
	// TODO prefer buffers with errors over those without errors
	copy(ret, aec.buffNames)
	return ret
}
