// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package hotreload

import (
	"context"

	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/languages"
	"namespacelabs.dev/foundation/internal/wscontents"
)

type hotReloadModule struct {
	module build.Workspace
	sink   wsremote.Sink
}

// If "triggerFullRebuildPredicate" returns true, a full rebuild will be triggered instead of a hot reload.
func NewHotReloadModule(module build.Workspace, opts *languages.HotReloadOpts) build.Workspace {
	return hotReloadModule{
		module: module,
		sink:   observerSink{opts.Sink, opts.TriggerFullRebuiltPredicate},
	}
}

func (w hotReloadModule) ModuleName() string { return w.module.ModuleName() }
func (w hotReloadModule) Abs() string        { return w.module.Abs() }
func (w hotReloadModule) VersionedFS(rel string, observeChanges bool) compute.Computable[wscontents.Versioned] {
	if observeChanges {
		return wsremote.ObserveAndPush(w.module.Abs(), rel, w.sink)
	}

	return w.module.VersionedFS(rel, observeChanges)
}

type observerSink struct {
	sink wsremote.Sink

	triggerFullRebuildPredicate func(filepath string) bool
}

func (obs observerSink) Deposit(ctx context.Context, events []*wscontents.FileEvent) (bool, error) {
	for _, ev := range events {
		if obs.triggerFullRebuildPredicate != nil && obs.triggerFullRebuildPredicate(ev.Path) {
			return false, nil
		}
	}

	return obs.sink.Deposit(ctx, events)
}
