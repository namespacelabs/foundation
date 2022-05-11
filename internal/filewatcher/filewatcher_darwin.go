// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

//go:build darwin
// +build darwin

package filewatcher

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsevents"
	"github.com/fsnotify/fsnotify"
	"go.uber.org/atomic"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
)

func NewFactory(ctx context.Context) (FileWatcherFactory, error) {
	return &fsEvents{}, nil
}

type fsEvents struct {
	files       []string
	directories []string
}

func (fsn *fsEvents) AddFile(name string) error {
	if !filepath.IsAbs(name) {
		return fmt.Errorf("%s: must be an absolute path", name)
	}

	fsn.files = append(fsn.files, name)
	return nil
}

func (fsn *fsEvents) AddDirectory(name string) error {
	if !filepath.IsAbs(name) {
		return fmt.Errorf("%s: must be an absolute path", name)
	}

	fsn.directories = append(fsn.directories, name)
	return nil
}

func (fsn *fsEvents) StartWatching(ctx context.Context) (EventsAndErrors, error) {
	var files uniquestrings.List
	var dirs uniquestrings.List

	for _, f := range fsn.files {
		files.Add(f)
		dirs.Add(filepath.Dir(f))
	}

	for _, d := range fsn.directories {
		dirs.Add(d)
	}

	root := longestCommonPrefix(dirs.Strings())
	if root == "" || root == "/" {
		return nil, fnerrors.New("fs notify common root is /, would watch too many files")
	}

	fmt.Fprintf(console.Debug(ctx), "fsevents: common root is %q\n", root)

	dev, err := fsevents.DeviceForPath(root)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve device: %w", err)
	}

	fmt.Fprintf(console.Debug(ctx), "fsevents: device is %v\n", dev)

	es := &fsevents.EventStream{
		Paths:   []string{root},
		Latency: 500 * time.Millisecond,
		Device:  dev,
		Flags:   fsevents.FileEvents | fsevents.WatchRoot,
	}

	es.Start()

	ch := make(chan fsnotify.Event)
	errCh := make(chan error)

	go func() {
		defer close(ch)
		defer close(errCh)

		for evs := range es.Events {
			for _, ev := range evs {
				fmt.Fprintf(console.Debug(ctx), "fsevents: event %v\n", ev)

				// Events are emitted without a leading root.
				realPath := "/" + ev.Path
				if !files.Has(realPath) && !dirs.Has(realPath) {
					continue
				}

				newEv := fsnotify.Event{Name: realPath}

				if (ev.Flags & fsevents.ItemModified) != 0 {
					newEv.Op = fsnotify.Write
				} else if (ev.Flags & fsevents.ItemRemoved) != 0 {
					newEv.Op = fsnotify.Remove
				} else if (ev.Flags & fsevents.ItemCreated) != 0 {
					newEv.Op = fsnotify.Create
				} else {
					fmt.Fprintf(console.Debug(ctx), "skipped fsevent: %s (flags %x)\n", realPath, ev.Flags)
					continue
				}

				fmt.Fprintf(console.Debug(ctx), "fsevents: emiting %v\n", newEv)

				ch <- newEv
			}
		}
	}()

	return &passEvents{es, ch, errCh, atomic.NewBool(false)}, nil
}

func (fsn *fsEvents) Close() error {
	return nil
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}

	sort.Strings(strs)
	first := strs[0]
	last := strs[len(strs)-1]

	longestPrefix := ""
	for i := 0; i < len(first); i++ {
		if last[i] != first[i] {
			break
		}

		longestPrefix += string(last[i])
	}

	return longestPrefix
}

type passEvents struct {
	es    *fsevents.EventStream
	ch    chan fsnotify.Event
	errCh chan error

	closed *atomic.Bool
}

func (p *passEvents) Events() <-chan fsnotify.Event { return p.ch }
func (p *passEvents) Errors() <-chan error          { return p.errCh }
func (p *passEvents) Close() error {
	if p.closed.CAS(false, true) {
		p.es.Stop()
		return nil
	}

	return fnerrors.New("already closed")
}
