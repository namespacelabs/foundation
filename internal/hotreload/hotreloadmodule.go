// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package hotreload

import (
	"context"
	"io/fs"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/wscontents"
)

type hotReloadModule struct {
	module build.Workspace
	sink   wsremote.Sink
}

func NewHotReloadModule(module build.Workspace, opts integrations.HotReloadOpts) build.Workspace {
	return hotReloadModule{
		module: module,
		sink:   observerSink{opts.Sink, opts.EventProcessor},
	}
}

func (w hotReloadModule) ModuleName() string             { return w.module.ModuleName() }
func (w hotReloadModule) Abs() string                    { return w.module.Abs() }
func (w hotReloadModule) ReadOnlyFS(rel ...string) fs.FS { return w.module.ReadOnlyFS(rel...) }

func (w hotReloadModule) ChangeTrigger(rel string) compute.Computable[compute.Versioned] {
	return compute.Transform("change-trigger", wsremote.ObserveAndPush(w.module.Abs(), rel, w.sink), func(_ context.Context, ws wscontents.Versioned) (compute.Versioned, error) {
		return ws, nil
	})
}

type observerSink struct {
	sink wsremote.Sink

	eventProcessor func(*wscontents.FileEvent) *wscontents.FileEvent
}

func (obs observerSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) (bool, error) {
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

	return obs.sink.Deposit(ctx, processedEvents)
}
