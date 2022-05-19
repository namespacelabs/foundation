// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/digestfs"
	"namespacelabs.dev/foundation/schema"
)

type FS struct {
	files []*fnfs.File
	root  fsNode
}

type fsNode struct {
	file     *fnfs.File
	children map[string]*fsNode
}

type FSStats struct {
	FileCount int
}

var _ fs.ReadDirFS = &FS{}
var _ fnfs.VisitFS = &FS{}
var _ fnfs.TotalSizeFS = &FS{}

func (m *FS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fnerrors.New("invalid name")}
	}

	nodeName, node := walk(&m.root, name, false)
	if node == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	// Is is a dir?
	if node.file == nil {
		return DirHandle(nodeName, readDir(node)), nil
	}

	return FileHandle(*node.file), nil
}

func (m *FS) ReadDir(relPath string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(relPath) {
		return nil, &fs.PathError{Op: "readdir", Path: relPath, Err: fnerrors.New("invalid name")}
	}

	_, node := walk(&m.root, relPath, false)
	if node == nil {
		return nil, &fs.PathError{Op: "readdir", Path: relPath, Err: fs.ErrNotExist}
	}

	if node.file != nil {
		return nil, &fs.PathError{Op: "readdir", Path: relPath, Err: fnerrors.New("is a regular file")}
	}

	return readDir(node), nil
}

func readDir(node *fsNode) []fs.DirEntry {
	var entries []fs.DirEntry
	for name, child := range node.children {
		if f := child.file; f == nil {
			entries = append(entries, dirDirent{Basename: name, DirMode: memDirMode})
		} else {
			entries = append(entries, FileDirent{Path: f.Path, ContentSize: int64(len(f.Contents)), FileMode: memFileMode})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return strings.Compare(entries[i].Name(), entries[j].Name()) < 0
	})
	return entries
}

func (m *FS) VisitFiles(ctx context.Context, visitor func(string, bytestream.ByteStream, fs.DirEntry) error) error {
	// Need to guarantee that VisitFiles calls visitor in a deterministic way.
	sorted := make([]*fnfs.File, len(m.files))
	copy(sorted, m.files)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.Compare(sorted[i].Path, sorted[j].Path) < 0
	})

	for _, f := range sorted {
		dirent := FileDirent{Path: f.Path, ContentSize: int64(len(f.Contents)), FileMode: memFileMode}
		if err := visitor(f.Path, bytestream.Static{Contents: f.Contents}, dirent); err != nil {
			return err
		}
	}
	return nil
}

func (m *FS) OpenWrite(name string, _ fs.FileMode) (fnfs.WriteFileHandle, error) {
	// XXX support filemode
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "write", Path: name, Err: fnerrors.New("invalid name")}
	}

	return &writeHandle{parent: m, path: name}, nil
}

func (m *FS) Remove(relPath string) error {
	if !fs.ValidPath(relPath) {
		return &fs.PathError{Op: "remove", Path: relPath, Err: fnerrors.New("invalid name")}
	}

	dirname := filepath.Dir(relPath)
	filename := filepath.Base(relPath)

	_, dir := walk(&m.root, dirname, false)
	if dir == nil {
		return &fs.PathError{Op: "remove", Path: relPath, Err: fs.ErrNotExist}
	}

	dirent, ok := dir.children[filename]
	if !ok {
		return &fs.PathError{Op: "remove", Path: relPath, Err: fs.ErrNotExist}
	}

	if dirent.file == nil && len(dirent.children) > 0 {
		return &fs.PathError{Op: "remove", Path: relPath, Err: fnerrors.New("directory is not empty")}
	}

	delete(dir.children, filename)
	return nil
}

func (m *FS) Clone() fnfs.ReadWriteFS {
	c := &FS{}
	c.files = make([]*fnfs.File, len(m.files))
	copy(c.files, m.files)
	c.root = *m.root.clone()
	return c
}

func (m *FS) ComputeDigest(ctx context.Context) (schema.Digest, error) {
	return digestfs.Digest(ctx, m)
}

func (m *FS) TotalSize(ctx context.Context) (uint64, error) {
	var count uint64
	for _, f := range m.files {
		count += uint64(len(f.Contents))
	}
	return count, nil
}

func (n *fsNode) clone() *fsNode {
	c := &fsNode{file: n.file, children: map[string]*fsNode{}}
	for k, v := range n.children {
		c.children[k] = v.clone()
	}
	return c
}

func walk(root *fsNode, path string, create bool) (string, *fsNode) {
	if path == "/" || path == "." {
		return "/", root
	}

	if path == "" {
		return "", nil
	}

	if path[0] == '/' {
		return walk(root, path[1:], create)
	}

	var search, sub string

	i := strings.IndexByte(path, '/')
	if i < 0 {
		search = path
		sub = ""
	} else {
		search = path[0:i]
		sub = path[(i + 1):]
	}

	child, ok := root.children[search]
	if create && !ok {
		if root.children == nil {
			root.children = map[string]*fsNode{}
		}
		child = &fsNode{}
		root.children[search] = child
	} else if !ok {
		return "", nil
	}

	if sub != "" {
		return walk(child, sub, create)
	}

	return search, child
}

func (m *FS) Add(path string, contents []byte) {
	file := &fnfs.File{Path: path, Contents: contents}
	_, node := walk(&m.root, path, true)
	node.file = file
	m.files = append(m.files, file)
}

func (m *FS) Stats() FSStats {
	return FSStats{FileCount: len(m.files)}
}

func FileHandle(f fnfs.File) fs.File {
	return memFile{entry: f, reader: bytes.NewReader(f.Contents)}
}

type memFile struct {
	entry  fnfs.File
	reader *bytes.Reader
}

func (mf memFile) Stat() (fs.FileInfo, error) {
	return FileDirent{Path: mf.entry.Path, ContentSize: int64(len(mf.entry.Contents)), FileMode: memFileMode}, nil
}

func (mf memFile) Read(p []byte) (int, error) {
	return mf.reader.Read(p)
}

func (mf memFile) Close() error { return nil }

func DirHandle(basename string, entries []fs.DirEntry) fs.ReadDirFile {
	return &memDir{basename, entries}
}

type memDir struct {
	basename string
	entries  []fs.DirEntry
}

func (md *memDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if n < 0 {
		return md.entries, nil
	}

	if len(md.entries) == 0 {
		return nil, io.EOF
	}

	if n > len(md.entries) {
		n = len(md.entries)
	}

	read := md.entries[0:n]
	left := md.entries[n:]

	md.entries = left
	return read, nil
}

func (md *memDir) Stat() (fs.FileInfo, error) {
	return dirDirent{Basename: md.basename}, nil
}

func (md *memDir) Read(p []byte) (int, error) {
	return 0, fs.ErrInvalid
}

func (md *memDir) Close() error { return nil }

type writeHandle struct {
	parent *FS
	path   string
	buffer bytes.Buffer
	closed bool
}

func (h *writeHandle) Write(p []byte) (n int, err error) {
	if h.closed {
		return 0, fs.ErrClosed
	}
	return h.buffer.Write(p)
}

func (h *writeHandle) Close() error {
	// XXX locking.
	if h.closed {
		return fs.ErrClosed
	}
	h.parent.Add(h.path, h.buffer.Bytes())
	h.closed = true
	return nil
}
