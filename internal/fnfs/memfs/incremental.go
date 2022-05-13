// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"context"
	"io/fs"
	"sync"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func IncrementalSnapshot(origin fs.FS) *IncrementalFS {
	return &IncrementalFS{origin: origin}
}

type IncrementalFS struct {
	origin   fs.FS
	mu       sync.RWMutex
	snapshot FS
}

func (inc *IncrementalFS) Open(name string) (fs.File, error) {
	inc.mu.RLock()
	// Is it already snapshotted?
	f, err := inc.snapshot.Open(name)
	inc.mu.RUnlock()

	if err == nil {
		return f, nil
	}

	inc.mu.Lock()
	defer inc.mu.Unlock()

	if err := fnfs.CopyFile(&inc.snapshot, name, inc.origin, name); err != nil {
		return nil, err
	}

	return inc.snapshot.Open(name)
}

func (inc *IncrementalFS) Snapshot(opts SnapshotOpts) (*FS, error) {
	return Snapshot(&inc.snapshot, opts)
}

func (inc *IncrementalFS) VisitFiles(ctx context.Context, f func(string, bytestream.ByteStream, fs.DirEntry) error) error {
	inc.mu.RLock()
	defer inc.mu.RUnlock()
	return inc.snapshot.VisitFiles(ctx, f)
}

func (inc *IncrementalFS) SnapshotDir(dir string, opts SnapshotOpts) (*FS, error) {
	return snapshotWith(inc.origin, opts, dir, false, func(path string) ([]byte, error) {
		return fs.ReadFile(inc, path)
	})
}

func (inc *IncrementalFS) Clone() fs.FS {
	return inc.snapshot.Clone()
}
