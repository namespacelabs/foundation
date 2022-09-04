// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package lsp

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path"
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"go.lsp.dev/uri"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/module"
)

// Represents an Fn workspace with its partially-parsed Cue files.
// An Fn workspace is rooted in a directory with a `workspace.ns.textpb“ file.
// There may be multiple [FnWorkspace]'s in an editor workspace.
// Roughly corresponds to a [workspace.Root], [schema.Workspace] and [loader.PackageLoader].
type FnWorkspace struct {
	root      *workspace.Root
	openFiles *OpenFiles // Paths (in URIs) are absolute!
	evalCtx   *fncue.EvalCtx
	env       planning.Context
}

func (s *server) WorkspaceForFile(ctx context.Context, absPath string) (ws *FnWorkspace, wsPath string, err error) {
	// NB: This reads FS directly, so we will detect a Namespace workspace only once workspace.ns.textpb is saved.
	root, loc, err := module.PackageAt(ctx, absPath)
	if err != nil {
		return nil, "", err
	}
	wsPath = loc.RelPath

	env, err := planning.LoadContext(root, "dev")
	if err != nil {
		return nil, "", err
	}

	ws = &FnWorkspace{
		root:      root,
		openFiles: s.openFiles,
		env:       env,
	}
	ws.evalCtx = fncue.NewEvalCtx(ws, cuefrontend.InjectedScope(env.Environment()))
	return
}

func (ws *FnWorkspace) AbsRoot() string {
	return ws.root.Abs()
}

// Take a relative path (package/file.cue) and returns a fully-qualified package name
// (example.com/module/package/file.cue) within the main workspace module.
func (ws *FnWorkspace) PkgNameInMainModule(relPath string) string {
	return path.Join(ws.root.Workspace().ModuleName, relPath)
}

// Real filesystem path for the package name (example.com/module/package/file.cue).
// Supports external modules and may download them on-demand (hence [ctx]).
func (ws *FnWorkspace) AbsPathForPkgName(ctx context.Context, pkgName string) (string, error) {
	packageLoader := workspace.NewPackageLoader(ws.env)
	loc, err := packageLoader.Resolve(ctx, schema.PackageName(pkgName))
	if err != nil {
		return "", err
	}
	return loc.Abs(), nil
}

func (ws *FnWorkspace) EvalPackage(ctx context.Context, pkgName string) (cue.Value, error) {
	value, err := ws.evalCtx.EvalPackage(ctx, pkgName)
	if err != nil && value == nil {
		// Retain Cue-level errors inside the "successful" value so that we can
		// provide exploration features while having errors.
		return cue.Value{}, err
	}
	return value.CueV.Val, nil
}

func (ws *FnWorkspace) FS() fs.ReadDirFS {
	return &workspaceFS{
		ReadDirFS: ws.root.ReadOnlyFS(),
		rootPath:  ws.root.Abs(),
		openFiles: ws.openFiles,
	}
}

func (ws *FnWorkspace) SnapshotDir(ctx context.Context, pkgname schema.PackageName, opts memfs.SnapshotOpts) (fnfs.Location, string, error) {
	packageLoader := workspace.NewPackageLoader(ws.env)

	loc, err := packageLoader.Resolve(ctx, pkgname) // This may download external modules.
	if err != nil {
		fmt.Fprintf(console.Warnings(ctx), "%s: resolve failed: %v\n", pkgname, err)
		return fnfs.Location{}, "", err
	}

	var fsys fs.FS
	if loc.Module.ModuleName() == ws.root.Workspace().ModuleName {
		// Module corresponding to the current Fn workspace.
		fsys = ws.FS()
	} else {
		w, err := packageLoader.WorkspaceOf(ctx, loc.Module) // This may download external modules.
		if err != nil {
			return fnfs.Location{}, "", err
		}

		fsys, err = w.SnapshotDir(loc.Rel(), opts)
		if err != nil {
			return fnfs.Location{}, "", err
		}
	}

	return fnfs.Location{
		ModuleName: loc.Module.ModuleName(),
		RelPath:    loc.Rel(),
		FS:         fsys,
	}, loc.Abs(), nil
}

// A [fnfs.ReadDirFS] that wraps an underlying FS and overlays with the open editor file contents with in-memory contents.
// This is useful to allow Fn infra to access open files in the editor.
type workspaceFS struct {
	fs.ReadDirFS

	// Absolute path to the root of [ReadDirFS]. Used to address files in [openFiles]
	rootPath  string
	openFiles *OpenFiles
}

func (f *workspaceFS) Open(pathInWorkspace string) (fs.File, error) {
	absPath := path.Join(f.rootPath, pathInWorkspace)
	if snapshot, err := f.openFiles.Read(uri.File(absPath)); err == nil {
		return memfs.FileHandle(fnfs.File{Path: pathInWorkspace, Contents: []byte(snapshot.Text)}), nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return f.ReadDirFS.Open(pathInWorkspace)
}

func WriteOverlay(underlying fs.ReadDirFS) *writeOverlay {
	return &writeOverlay{
		ReadDirFS: underlying,
		writes:    make(map[string][]byte),
	}
}

type writeOverlay struct {
	fs.ReadDirFS

	mu     sync.Mutex
	writes map[string][]byte
}

func (f *writeOverlay) AllWrites() map[string][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.writes
}

func (f *writeOverlay) OpenWrite(path string, mode fs.FileMode) (fnfs.WriteFileHandle, error) {
	buf := new(bytes.Buffer)
	return &editorFSWriteHandle{
		Buffer: buf,
		fs:     f,
		path:   path,
	}, nil
}

func (f *writeOverlay) Remove(path string) error {
	return fs.ErrInvalid
}

type editorFSWriteHandle struct {
	*bytes.Buffer
	fs   *writeOverlay
	path string
}

func (h *editorFSWriteHandle) Close() error {
	h.fs.mu.Lock()
	defer h.fs.mu.Unlock()

	h.fs.writes[h.path] = h.Buffer.Bytes()
	return nil
}
