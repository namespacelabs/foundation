// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package wsremote

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Sink interface {
	Deposit(context.Context, []*wscontents.FileEvent) error
}

// Returns a wscontents.Versioned which will produce a local snapshot as expected
// but forwards all filesystem events (e.g. changes, removals) to the specified sink.
func ObserveAndPush(absPath, rel string, sink Sink) compute.Computable[wscontents.Versioned] {
	return &observePath{absPath: absPath, rel: rel, sink: sink}
}

type observePath struct {
	absPath string
	rel     string

	sink Sink

	compute.DoScoped[wscontents.Versioned]
}

func (op *observePath) Action() *tasks.ActionEvent {
	return tasks.Action("web.contents.observe")
}
func (op *observePath) Inputs() *compute.In {
	return compute.Inputs().Str("absPath", op.absPath).Str("rel", op.rel)
}
func (op *observePath) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}
func (op *observePath) Compute(ctx context.Context, _ compute.Resolved) (wscontents.Versioned, error) {
	absPath := filepath.Join(op.absPath, op.rel)

	snapshot, err := wscontents.SnapshotDirectory(absPath)
	if err != nil {
		return nil, err
	}

	return localObserver{absPath: absPath, snapshot: snapshot, sink: op.sink}, nil
}

type localObserver struct {
	absPath  string
	snapshot *memfs.FS
	sink     Sink
}

func (lo localObserver) Abs() string { return lo.absPath }
func (lo localObserver) FS() fs.FS   { return lo.snapshot }
func (lo localObserver) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return digestfs.Digest(ctx, lo.snapshot)
}

func (lo localObserver) Observe(ctx context.Context, _ func(compute.ResultWithTimestamp[any], bool)) (func(), error) {
	// XXX we're doing polling for correctness; this needs to use filewatcher.

	// This observer is special; if we know that the scheduler wants to observe
	// the graph, then we trigger a syncing of local files to the destination
	// sink. We don't actually ever emit a new snapshot.

	closeCh := make(chan struct{})
	last := lo.snapshot

	go func() {
		for {
			select {
			case <-closeCh:
				return
			case <-time.After(time.Second):
				newSnapshot, err := checkSnapshot(ctx, last, lo.absPath, lo.sink)
				if err != nil {
					fmt.Fprintf(console.Errors(ctx), "FileSync failed while snapshotting %q: %v\n", lo.absPath, err)
					return
				}

				last = newSnapshot
			}
		}
	}()

	return func() { close(closeCh) }, nil
}

func checkSnapshot(ctx context.Context, previous *memfs.FS, absPath string, sink Sink) (*memfs.FS, error) {
	newSnapshot, err := wscontents.SnapshotDirectory(absPath)
	if err != nil {
		return nil, err
	}

	// First we iterate over all files in the new snapshot. This index will be
	// used to signal which files are actually new, and which files were removed
	// from the old snapshot.
	newFiles := map[string]bytestream.Static{}
	newFilesModes := map[string]memfs.FileDirent{}
	if err := newSnapshot.VisitFilesWithoutContext(func(path string, bs bytestream.Static, de memfs.FileDirent) error {
		newFiles[path] = bs
		newFilesModes[path] = de
		return nil
	}); err != nil {
		return nil, err
	}

	var events []*wscontents.FileEvent

	if err := previous.VisitFilesWithoutContext(func(path string, bs bytestream.Static, _ memfs.FileDirent) error {
		if newFile, ok := newFiles[path]; !ok {
			events = append(events, &wscontents.FileEvent{Path: path, Event: wscontents.FileEvent_REMOVE})
			delete(newFiles, path)
		} else {
			if bytes.Equal(bs.Contents, newFile.Contents) {
				// No changes, don't re-write.
				delete(newFiles, path)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	m := checkMkdir{previous: previous, newdirs: map[string]struct{}{}}

	for filename, contents := range newFiles {
		if err := m.check(filepath.Dir(filename)); err != nil {
			return nil, err
		}

		events = append(events, &wscontents.FileEvent{
			Event:       wscontents.FileEvent_WRITE,
			Path:        filename,
			NewContents: contents.Contents,
			Mode:        uint32(newFilesModes[filename].FileMode.Perm()),
		})
	}

	// Mkdirs come first.
	events = append(m.events, events...)

	if len(events) > 0 {
		if err := sink.Deposit(ctx, events); err != nil {
			return nil, err
		}
	}

	return newSnapshot, nil
}

type checkMkdir struct {
	previous *memfs.FS
	newdirs  map[string]struct{}
	events   []*wscontents.FileEvent
}

// XXX this doesn't handle the case where a directory becomes a file, or vice-versa.
func (m *checkMkdir) check(dir string) error {
	if dir == "." {
		return nil
	}

	if err := m.check(filepath.Dir(dir)); err != nil {
		return err
	}

	if _, ok := m.newdirs[dir]; ok {
		return nil
	}

	if _, err := m.previous.Open(dir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		m.events = append(m.events, &wscontents.FileEvent{Path: dir, Event: wscontents.FileEvent_MKDIR})
		m.newdirs[dir] = struct{}{}
	}

	return nil
}
