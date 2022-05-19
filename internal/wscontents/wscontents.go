// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package wscontents

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/filewatcher"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Observe returns a Computable that is Versioned, that is, it produces a value
// but can also be observed for updates. It produces a new revision whenever the
// filesystem is updated.
// Returns a Computable[Versioned]
func Observe(absPath, rel string, observeChanges bool) compute.Computable[Versioned] {
	return &observePath{absPath: absPath, rel: rel, observeChanges: observeChanges}
}

type computeContents struct {
	absPath string
	compute.DoScoped[fs.FS]
}

var _ compute.Computable[fs.FS] = &computeContents{}

func (cc *computeContents) Action() *tasks.ActionEvent {
	return tasks.Action("module.contents.load").Arg("absPath", cc.absPath)
}
func (cc *computeContents) Inputs() *compute.In {
	return compute.Inputs().Str("absPath", cc.absPath)
}
func (cc *computeContents) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (cc *computeContents) Compute(ctx context.Context, _ compute.Resolved) (fs.FS, error) {
	inmem, err := SnapshotContents(ctx, cc.absPath, ".")
	if err != nil {
		return nil, err
	}

	return &contentFS{absPath: cc.absPath, fs: inmem}, nil
}

func SnapshotContents(ctx context.Context, modulePath, rel string) (fsys *memfs.FS, err error) {
	err = tasks.Action("module.contents.snapshot").Arg("absPath", modulePath).Arg("rel", rel).Run(ctx, func(ctx context.Context) error {
		var err error
		fsys, err = snapshotContents(modulePath, rel)
		if err != nil {
			return err
		}
		att := tasks.Attachments(ctx)
		att.AddResult("fs.stats", fsys.Stats())
		if ts, err := fsys.TotalSize(ctx); err == nil {
			att.AddResult("fs.totalSize", humanize.Bytes(ts))
		}
		return nil
	})
	return
}

func verifyDir(path string) error {
	if st, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fnerrors.UserError(nil, "%s: accessing the path failed: %v", path, err)
	} else if !st.IsDir() {
		return fnerrors.UserError(nil, "%s: expected it to be a directory", path)
	}

	return nil
}

func snapshotContents(modulePath, rel string) (*memfs.FS, error) {
	if err := verifyDir(modulePath); err != nil {
		return nil, err
	}

	absPath := filepath.Join(modulePath, rel)

	if rel != "." {
		if err := verifyDir(absPath); err != nil {
			return nil, err
		}
	}

	var inmem memfs.FS
	return &inmem, filepath.WalkDir(absPath, func(osPathname string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		name := de.Name()
		if len(name) == 0 {
			return filepath.SkipDir
		} else if name[0] == '.' { // Skip hidden directories.
			return filepath.SkipDir
		} else if slices.Contains(dirs.DirsToAvoid, name) {
			return filepath.SkipDir
		}

		if !de.Type().IsRegular() {
			return nil
		}

		rel, err := filepath.Rel(absPath, osPathname) // XXX expensive?
		if err != nil {
			return err
		}

		f, err := os.Open(osPathname)
		if err != nil {
			return err
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			return err
		}

		d, err := inmem.OpenWrite(rel, st.Mode().Perm())
		if err != nil {
			return err
		}

		_, err = io.Copy(d, f)
		err2 := d.Close()

		if err != nil {
			return err
		}

		return err2
	})
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
	snapshot, err := SnapshotContents(ctx, moduleAbsPath, rel)
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
		errLogger:      console.Output(ctx, "observepath"),
		onNewSnapshot:  onNewSnapshot,
		observeChanges: observeChanges,
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

func (vp *versioned) Observe(ctx context.Context, onChange func(compute.ResultWithTimestamp[any], bool)) (func(), error) {
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

	if err := fs.WalkDir(vp.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if (path != "." && path[0] == '.') || slices.Contains(dirs.DirsToAvoid, d.Name()) {
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
		return nil, err
	}

	// We keep a buffer of N events over a time window, to avoid too much churn.
	bufferCh := make(chan []fsnotify.Event)
	go func() {
		for buffer := range bufferCh {
			newFsys, deliver, err := handleEvents(ctx, console.Debug(ctx), vp.errLogger, vp.absPath, fsys, vp.onNewSnapshot, buffer)
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
				Timestamp: time.Now(),
			}
			vp.revision++
			r.Value = &versioned{
				absPath:        vp.absPath,
				fs:             fsys,
				revision:       vp.revision,
				errLogger:      vp.errLogger,
				observeChanges: vp.observeChanges,
			}
			onChange(r, false)
		}

		for range bufferCh {
			// Drain the channel in case we escaped the loop above, so the go-routine below
			// has a chance to observe a canceled context and close the channel.
		}
	}()

	w, err := watcher.StartWatching(ctx)
	if err != nil {
		return nil, err
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

func handleEvents(ctx context.Context, debugLogger, userVisible io.Writer, absPath string, fsys fnfs.ReadWriteFS, onNewSnapshot OnNewSnapshopFunc, buffer []fsnotify.Event) (fnfs.ReadWriteFS, bool, error) {
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

		action, err := checkChanges(fsys, os.DirFS(absPath), rel)
		if err != nil {
			return nil, false, fnerrors.InternalError("failed to check changes: %v", err)
		}
		if action != nil {
			actions = append(actions, action)
		}
	}

	if len(actions) == 0 {
		return fsys, false, nil
	}

	var labels []string
	for _, p := range actions {
		labels = append(labels, fmt.Sprintf("%s:%s", p.Event, p.Path))

		var err error
		switch p.Event {
		case FileEvent_WRITE:
			err = fnfs.WriteFile(ctx, fsys, p.Path, p.NewContents, fs.FileMode(p.Mode))
		case FileEvent_REMOVE:
			err = fsys.Remove(p.Path)
		case FileEvent_MKDIR:
			if mkdirfs, ok := fsys.(fnfs.MkdirFS); ok {
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
		return fsys, true, nil
	}

	return onNewSnapshot(ctx, fsys, actions)
}

func checkChanges(snapshot fs.FS, ws fs.FS, path string) (*FileEvent, error) {
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
		return &FileEvent{Event: FileEvent_MKDIR, Path: path, Mode: uint32(st.Mode().Perm())}, nil
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	existingContents, err := fs.ReadFile(snapshot, path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil && bytes.Equal(contents, existingContents) {
		return nil, nil
	}

	return &FileEvent{Event: FileEvent_WRITE, Path: path, NewContents: contents, Mode: uint32(st.Mode().Perm())}, nil
}
