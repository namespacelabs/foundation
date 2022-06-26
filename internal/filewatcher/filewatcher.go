// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package filewatcher

import (
	"context"

	"github.com/fsnotify/fsnotify"
)

var NewFactory func(ctx context.Context) (FileWatcherFactory, error)

type FileWatcherFactory interface {
	AddFile(name string) error
	AddDirectory(name string) error
	StartWatching(context.Context) (EventsAndErrors, error)
	Close() error
}

type EventsAndErrors interface {
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Close() error
}
