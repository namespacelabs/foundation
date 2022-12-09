// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package hotreload

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/std/tasks"
)

type devModule struct {
	module         build.Workspace
	observeChanges bool
	sink           wsremote.Sink
	extraInputs    []compute.UntypedComputable
}

func NewDevModule(module build.Workspace, observeChanges bool, opts integrations.HotReloadOpts, extraInputs ...compute.UntypedComputable) build.Workspace {
	return devModule{
		module:         module,
		observeChanges: observeChanges,
		sink:           observerSink{opts.Sink, opts.EventProcessor},
		extraInputs:    extraInputs,
	}
}

func (w devModule) ModuleName() string             { return w.module.ModuleName() }
func (w devModule) Abs() string                    { return w.module.Abs() }
func (w devModule) ReadOnlyFS(rel ...string) fs.FS { return w.module.ReadOnlyFS(rel...) }

func (w devModule) ChangeTrigger(rel string) compute.Computable[any] {
	if !w.observeChanges {
		in := compute.Inputs()
		for k, extra := range w.extraInputs {
			in = in.Computable(fmt.Sprintf("extra:%d", k), extra)
		}

		return compute.Map(tasks.Action("dev.trigger-actions"), in, compute.Output{NotCacheable: true}, func(ctx context.Context, r compute.Resolved) (any, error) {
			return "no action", nil
		})
	}

	return wsremote.ObserveAndPush(filepath.Join(w.module.Abs(), rel), w.sink, w.extraInputs...)
}

type observerSink struct {
	sink wsremote.Sink

	eventProcessor func(*wscontents.FileEvent) *wscontents.FileEvent
}

func (obs observerSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) (bool, error) {
	fmt.Fprintf(console.Debug(ctx), "deposit\n")

	processedEvents := []*wscontents.FileEvent{}
	for _, ev := range events {
		processedEvent := ev
		if obs.eventProcessor != nil {
			processedEvent = obs.eventProcessor(ev)
			if processedEvent == nil {
				return false, nil
			}
		}
		processedEvents = append(processedEvents, processedEvent)
	}

	if obs.sink == nil {
		return false, nil
	}

	return obs.sink.Deposit(ctx, processedEvents)
}
