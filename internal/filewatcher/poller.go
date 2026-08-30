// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package filewatcher

import (
	"context"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/go-filenotify"
)

var FileWatcherUsePolling bool

func SetupFileWatcher() {
	if FileWatcherUsePolling || NewFactory == nil {
		NewFactory = NewPollingFactory
	}
}

func NewPollingFactory(ctx context.Context) (FileWatcherFactory, error) {
	return fsNotifyWrapper{filenotify.NewPollingWatcher(console.Debug(ctx))}, nil
}

type fsNotifyWrapper struct {
	fw filenotify.FileWatcher
}

func (fsn fsNotifyWrapper) AddFile(name string) error {
	return fsn.fw.Add(name)
}

func (fsn fsNotifyWrapper) AddDirectory(name string) error {
	return fsn.fw.Add(name)
}

func (fsn fsNotifyWrapper) StartWatching(context.Context) (EventsAndErrors, error) {
	return fsn.fw, nil
}

func (fsn fsNotifyWrapper) Close() error {
	return fsn.fw.Close()
}
