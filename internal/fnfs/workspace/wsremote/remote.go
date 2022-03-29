// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package wsremote

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Sink interface {
	Deposit(context.Context, []*wscontents.FileEvent) error
}

// Returns a wscontents.Versioned which will produce a local snapshot as expected
// but forwards all filesystem events (e.g. changes, removals) to the specified sink.
func ObserveAndPush(absPath, rel string, sink Sink) compute.Computable[wscontents.Versioned] {
	return &observePath{absPath: absPath, rel: rel, sink: sink}
}

type observePath struct {
	absPath string
	rel     string

	sink Sink

	compute.DoScoped[wscontents.Versioned]
}

func (op *observePath) Action() *tasks.ActionEvent {
	return tasks.Action("web.contents.observe")
}
func (op *observePath) Inputs() *compute.In {
	return compute.Inputs().Str("absPath", op.absPath).Str("rel", op.rel)
}
func (op *observePath) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (op *observePath) Compute(ctx context.Context, _ compute.Resolved) (wscontents.Versioned, error) {
	return wscontents.MakeVersioned(ctx, op.absPath, op.rel, true, func(ctx context.Context, fsys fnfs.ReadWriteFS, events []*wscontents.FileEvent) (fnfs.ReadWriteFS, bool, error) {
		if len(events) > 0 {
			err := op.sink.Deposit(ctx, events)
			// Don't deliver the original versioned event.
			return fsys, false, err
		}

		return fsys, true, nil
	})
}