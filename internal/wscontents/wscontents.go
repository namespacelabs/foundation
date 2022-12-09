// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package wscontents

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/dustin/go-humanize"
	"github.com/fsnotify/fsnotify"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/filewatcher"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

// Observe returns a Computable that is Versioned, that is, it produces a value
// but can also be observed for updates. It produces a new revision whenever the
// filesystem is updated.
// Returns a Computable[Versioned]
func Observe(absPath, rel string, observeChanges bool) compute.Computable[Versioned] {
	return &observePath{absPath: absPath, rel: rel, observeChanges: observeChanges}
}

func snapshotContents(ctx context.Context, modulePath, rel string, digestMode bool) (*memfs.FS, error) {
	return tasks.Return(ctx, tasks.Action("module.contents.snapshot").Arg("absPath", modulePath).Arg("rel", rel), func(ctx context.Context) (*memfs.FS, error) {
		if err := verifyDir(modulePath); err != nil {
			return nil, err
		}

		fsys, err := snapshotDirectory(ctx, filepath.Join(modulePath, rel), digestMode)
		if err != nil {
			return nil, fnerrors.InternalError("snapshot failed: %v", err)
		}

		att := tasks.Attachments(ctx)
		att.AddResult("fs.stats", fsys.Stats())
		if !digestMode {
			if ts, err := fsys.TotalSize(ctx); err == nil {
				att.AddResult("fs.totalSize", humanize.Bytes(ts))
			}
		}

		return fsys, nil
	})
}

func verifyDir(path string) error {
	if st, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fnerrors.New("%s: accessing the path failed: %v", path, err)
	} else if !st.IsDir() {
		return fnerrors.New("%s: expected it to be a directory", path)
	}

	return nil
}

func SnapshotDirectory(ctx context.Context, absPath string) (*memfs.FS, error) {
	fsys, err := snapshotDirectory(ctx, absPath, false)
	if err != nil {
		return nil, fnerrors.InternalError("snapshot failed: %v", err)
	}
	return fsys, nil
}

func snapshotDirectory(ctx context.Context, absPath string, digest bool) (*memfs.FS, error) {
	if err := verifyDir(absPath); err != nil {
		return nil, err
	}

	var inmem memfs.FS
	if err := filepath.WalkDir(absPath, func(osPathname string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// TODO: use the exclude/include patterns passed as a config instead.
		if dirs.IsExcludedAsSource(osPathname) {
			if de.IsDir() {
				return filepath.SkipDir
			}

			return nil
		}

		if de.IsDir() {
			return nil
		} else if !de.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(absPath, osPathname) // XXX expensive?
		if err != nil {
			return err
		}

		contents, err := os.Open(osPathname)
		if err != nil {
			return err
		}
		defer contents.Close()

		st, err := contents.Stat()
		if err != nil {
			return err
		}

		// XXX symlinks.
		if !st.Mode().IsRegular() {
			return nil
		}

		target, err := inmem.OpenWrite(rel, st.Mode().Perm())
		if err != nil {
			return err
		}

		// In digest mode, we store a digest of the file as the file contents.
		if digest {
			var digest []byte
			digest, err = digestfile(contents)
			if err == nil {
				_, err = target.Write(digest)
			}
		} else {
			_, err = io.Copy(target, contents)
		}

		err2 := target.Close()

		if err != nil {
			return err
		}

		return err2
	}); err != nil {
		return nil, err
	}

	return &inmem, nil
}

type contentFS struct {
	absPath string
	fs      *memfs.FS
}

var _ fs.ReadDirFS = &contentFS{}
var _ fnfs.VisitFS = &contentFS{}

func (fsys *contentFS) Open(name string) (fs.File, error) {
	return fsys.fs.Open(name)
}

func (fsys *contentFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fsys.fs.ReadDir(name)
}

func (fsys *contentFS) VisitFiles(ctx context.Context, f func(string, bytestream.ByteStream, fs.DirEntry) error) error {
	return fsys.fs.VisitFiles(ctx, f)
}

type observePath struct {
	absPath        string
	rel            string
	observeChanges bool

	compute.DoScoped[Versioned]
}

func (op *observePath) Action() *tasks.ActionEvent {
	return tasks.Action("module.contents.observe")
}
func (op *observePath) Inputs() *compute.In {
	return compute.Inputs().Str("absPath", op.absPath).Str("rel", op.rel).Bool("observeChanges", op.observeChanges)
}
func (op *observePath) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (op *observePath) Compute(ctx context.Context, _ compute.Resolved) (Versioned, error) {
	return MakeVersioned(ctx, op.absPath, op.rel, op.observeChanges, nil)
}

// onNewSnapshot is guaranteed to never be called concurrently.
func MakeVersioned(ctx context.Context, moduleAbsPath, rel string, observeChanges bool, onNewSnapshot OnNewSnapshopFunc) (Versioned, error) {
	return makeVersioned(ctx, moduleAbsPath, rel, observeChanges, false, onNewSnapshot)
}

func makeVersioned(ctx context.Context, moduleAbsPath, rel string, observeChanges, digestMode bool, onNewSnapshot OnNewSnapshopFunc) (Versioned, error) {
	snapshot, err := snapshotContents(ctx, moduleAbsPath, rel, digestMode)
	if err != nil {
		return nil, err
	}

	var fsys fnfs.ReadWriteFS = snapshot
	if onNewSnapshot != nil {
		fsys, _, err = onNewSnapshot(ctx, fsys, nil)
		if err != nil {
			return nil, err
		}
	}

	return &versioned{
		absPath:        filepath.Join(moduleAbsPath, rel),
		fs:             fsys,
		revision:       1,
		errLogger:      console.Output(ctx, "file-observer"),
		onNewSnapshot:  onNewSnapshot,
		observeChanges: observeChanges,
		digestMode:     digestMode,
	}, nil
}

type Versioned interface {
	compute.Versioned
	compute.Digestible
	Abs() string
	FS() fs.FS
}

type OnNewSnapshopFunc func(context.Context, fnfs.ReadWriteFS, []*FileEvent) (fnfs.ReadWriteFS, bool, error)

type versioned struct {
	absPath        string
	fs             fs.FS
	revision       uint64
	errLogger      io.Writer
	observeChanges bool
	digestMode     bool

	// If available, is called on the updated snapshot before sending an update.
	onNewSnapshot OnNewSnapshopFunc
}

var _ compute.Versioned = &versioned{}

func (vp *versioned) Abs() string {
	return vp.absPath
}

func (vp *versioned) FS() fs.FS {
	return vp.fs
}

func (vp *versioned) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return digestfs.Digest(ctx, vp.fs)
}

func (vp *versioned) Observe(ctx context.Context, onChange func(compute.ResultWithTimestamp[any], compute.ObserveNote)) (func(), error) {
	if !vp.observeChanges {
		return nil, nil
	}

	// Start with the original snapshot, and with each bundle of events, modify clones of it.
	var fsys fnfs.ReadWriteFS

	if snapshot, ok := vp.fs.(*memfs.FS); !ok {
		return nil, fnerrors.InternalError("expected fs to be a mem snapshot")
	} else {
		fsys = snapshot.Clone()
	}

	// XXX we could have an observe model driven from a single watcher, but
	// there's a new watcher instantiated per Observe for simplicity for now.

	watcher, err := filewatcher.NewFactory(ctx)
	if err != nil {
		return nil, err
	}

	if err := fnfs.WalkDir(vp.fs, ".", func(path string, d fs.DirEntry) error {
		// TODO: use the exclude/include patterns passed as a config instead.
		if dirs.IsExcludedAsSource(path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Watch every single file and directory in the snapshot.
		if d.Type().IsDir() {
			return watcher.AddDirectory(filepath.Join(vp.absPath, path))
		} else if d.Type().IsRegular() {
			return watcher.AddFile(filepath.Join(vp.absPath, path))
		}

		return nil
	}); err != nil {
		fmt.Fprintln(vp.errLogger, "watching failed: ", err)
		watcher.Close()
		return nil, fnerrors.InternalError("failed to walkdir: %v", err)
	}

	// We keep a buffer of N events over a time window, to avoid too much churn.
	bufferCh := make(chan []fsnotify.Event)
	go func() {
		for buffer := range bufferCh {
			newFsys, deliver, err := handleEvents(ctx, console.Debug(ctx), vp.errLogger, vp.absPath, fsys, vp.digestMode, vp.onNewSnapshot, buffer)
			if err != nil {
				compute.Stop(ctx, err)
				break
			}

			fsys = newFsys
			if !deliver {
				// The observer decided to handle actions on their own. But we
				// continue to maintain a snapshot with all updates, in case the
				// observer changes their mind.
				continue
			}

			r := compute.ResultWithTimestamp[any]{
				Completed: time.Now(),
			}
			vp.revision++
			r.Value = &versioned{
				absPath:        vp.absPath,
				fs:             fsys,
				revision:       vp.revision,
				errLogger:      vp.errLogger,
				observeChanges: vp.observeChanges,
			}
			onChange(r, compute.ObserveContinuing)
		}

		for range bufferCh {
			// Drain the channel in case we escaped the loop above, so the go-routine below
			// has a chance to observe a canceled context and close the channel.
		}
	}()

	w, err := watcher.StartWatching(ctx)
	if err != nil {
		return nil, fnerrors.InternalError("failed to start watcher: %v", err)
	}

	go AggregateFSEvents(w, console.Debug(ctx), vp.errLogger, bufferCh)

	return func() {
		w.Close()
	}, nil
}

func AggregateFSEvents(watcher filewatcher.EventsAndErrors, debugLogger, errLogger io.Writer, bufferCh chan []fsnotify.Event) {
	// Usually the return callback would be sole responsible to stop the watcher,
	// but we want to free resources as early as we know that we can longer listen
	// to events.
	defer watcher.Close()
	defer close(bufferCh)

	t := time.NewTicker(250 * time.Millisecond)
	defer func() {
		t.Stop()
	}()

	var buffer []fsnotify.Event
	for {
		select {
		case ev, ok := <-watcher.Events():
			if !ok {
				return
			}

			fmt.Fprintf(debugLogger, "Received filesystem event (%s): %v\n", ev.Name, ev.Op)
			buffer = append(buffer, ev)

		case _, ok := <-t.C:
			if ok && len(buffer) > 0 {
				bufferCh <- buffer
				buffer = nil
			}

		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}

			fmt.Fprintf(errLogger, "Received filesystem event error: %v\n", err)
		}
	}
}

func handleEvents(ctx context.Context, debugLogger, userVisible io.Writer, absPath string, snapshot fnfs.ReadWriteFS, digestMode bool, onNewSnapshot OnNewSnapshopFunc, buffer []fsnotify.Event) (fnfs.ReadWriteFS, bool, error) {
	// Coalesce multiple changes.
	var dirtyPaths uniquestrings.List
	for _, ev := range buffer {
		dirtyPaths.Add(ev.Name)
	}

	fmt.Fprintf(debugLogger, "Coalesced: %v\n", dirtyPaths.Strings())

	var actions []*FileEvent
	for _, p := range dirtyPaths.Strings() {
		// Ignore changes to .swp files (i.e. Vim swap files).
		if filepath.Ext(p) == ".swp" && filepath.Base(p)[0] == '.' {
			continue
		}

		rel, err := filepath.Rel(absPath, p)
		if err != nil {
			return nil, false, fnerrors.InternalError("rel failed: %v", err)
		}

		// Observed a change that is not within the destination we're observing. We'll ignore it.
		if strings.HasPrefix(rel, "../") {
			fmt.Fprintf(debugLogger, "ignored change: %s\n", rel)
			continue
		}

		action, err := checkChanges(ctx, snapshot, os.DirFS(absPath), rel, digestMode)
		if err != nil {
			return nil, false, fnerrors.InternalError("failed to check changes: %v", err)
		}
		if action != nil {
			actions = append(actions, action)
		}
	}

	if len(actions) == 0 {
		return snapshot, false, nil
	}

	var labels []string
	for _, p := range actions {
		labels = append(labels, fmt.Sprintf("%s:%s", p.Event, p.Path))

		var err error
		switch p.Event {
		case FileEvent_WRITE:
			err = fnfs.WriteFile(ctx, snapshot, p.Path, p.NewContents, fs.FileMode(p.Mode))
		case FileEvent_REMOVE:
			err = snapshot.Remove(p.Path)
		case FileEvent_MKDIR:
			if mkdirfs, ok := snapshot.(fnfs.MkdirFS); ok {
				err = mkdirfs.MkdirAll(p.Path, fs.FileMode(p.Mode))
			}
		default:
			err = fnerrors.InternalError("don't know how to handle %q", p.Event)
		}
		if err != nil {
			return nil, false, fnerrors.InternalError("failed to apply changes to %s: %v", p.Path, err)
		}
	}

	fmt.Fprintf(userVisible, "Detected changes: %v\n", labels)

	if onNewSnapshot == nil {
		return snapshot, true, nil
	}

	return onNewSnapshot(ctx, snapshot, actions)
}

func checkChanges(ctx context.Context, snapshot fs.FS, ws fs.FS, path string, digestMode bool) (*FileEvent, error) {
	f, err := ws.Open(path)
	if os.IsNotExist(err) {
		if _, err := fs.Stat(snapshot, path); os.IsNotExist(err) {
			return nil, nil
		}

		return &FileEvent{Event: FileEvent_REMOVE, Path: path}, nil
	} else if err != nil {
		return nil, err
	}

	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if st.IsDir() {
		fi, err := fs.Stat(snapshot, path)
		if err != nil {
			if os.IsNotExist(err) {
				return &FileEvent{Event: FileEvent_MKDIR, Path: path, Mode: uint32(st.Mode().Perm())}, nil
			}

			return nil, err
		}

		if fi.IsDir() {
			return nil, nil
		}

		return nil, fnerrors.New("%s: inconsistent event, is a directory in the local workspace but not in the snapshot", path)
	}

	var contents []byte
	if digestMode {
		contents, err = digestfile(f)
	} else {
		contents, err = io.ReadAll(f)
	}

	if err != nil {
		return nil, err
	}

	existingContents, err := fs.ReadFile(snapshot, path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil && bytes.Equal(contents, existingContents) {
		return nil, nil
	}

	ev := &FileEvent{Event: FileEvent_WRITE, Path: path, Mode: uint32(st.Mode().Perm())}
	if !digestMode {
		ev.NewContents = contents
	}

	return ev, nil
}

func digestfile(contents io.Reader) ([]byte, error) {
	h := xxhash.New()
	_, err := io.Copy(h, contents)
	return []byte(hex.EncodeToString(h.Sum(nil))), err
}
