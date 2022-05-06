// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

//go:build !darwin
// +build !darwin

package filewatcher

import (
	"context"

	"namespacelabs.dev/go-filenotify"
)

func NewFactory() (FileWatcherFactory, error) {
	return fsNotifyWrapper{filenotify.NewPollingWatcher()}, nil
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
