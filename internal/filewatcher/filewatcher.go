// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package filewatcher

import (
	"context"
	"sort"
	"strings"

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

func longestCommonPathPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}

	sort.Strings(strs)
	first := strings.Split(strs[0], "/")
	last := strings.Split(strs[len(strs)-1], "/")

	longestPrefix := []string{}
	for i := 0; i < len(first); i++ {
		if last[i] != first[i] {
			break
		}

		longestPrefix = append(longestPrefix, last[i])
	}

	return strings.Join(longestPrefix, "/")
}
