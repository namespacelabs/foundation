// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tarfs

import (
	"archive/tar"
	"context"
	"io"
	"io/fs"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
)

type TarStreamFunc func() (io.ReadCloser, error)

type FS struct {
	TarStream TarStreamFunc
}

var _ fs.ReadDirFS = FS{}
var _ fnfs.VisitFS = FS{}

func (l FS) Open(path string) (fs.File, error) {
	if !fs.ValidPath(path) {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fnerrors.New("invalid path")}
	}

	var inmem memfs.FS
	readFile, err := ReadFilesInto(&inmem, l.TarStream, checkPath(path))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}

	if len(readFile) == 0 {
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}

	return inmem.Open(path)
}

type checkPath string

func (c checkPath) Has(p string) bool { return p == string(c) }

func (l FS) ReadDir(dir string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(dir) {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fnerrors.New("invalid path")}
	}

	f, err := l.TarStream()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dir = filepath.Clean(dir)

	var dirents []fs.DirEntry

	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fnerrors.BadInputError("unexpected error: %v", err)}
		}

		if filepath.Dir(h.Name) != dir {
			continue
		}

		switch h.Typeflag {
		case tar.TypeReg:
			dirents = append(dirents, tarDirent{h.FileInfo()})

		case tar.TypeDir:
			if h.Name != "." {
				dirents = append(dirents, tarDirent{h.FileInfo()})
			}
		}
	}

	return dirents, nil
}

type tarDirent struct{ info fs.FileInfo }

func (de tarDirent) Name() string               { return de.info.Name() }
func (de tarDirent) IsDir() bool                { return de.info.IsDir() }
func (de tarDirent) Type() fs.FileMode          { return de.info.Mode().Type() }
func (de tarDirent) Info() (fs.FileInfo, error) { return de.info, nil }

func (l FS) VisitFiles(ctx context.Context, visitor func(string, []byte, fs.DirEntry) error) error {
	f, err := l.TarStream()
	if err != nil {
		return err
	}

	defer f.Close()

	tr := tar.NewReader(f)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		h, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if h.Typeflag != tar.TypeReg {
			continue
		}

		b, err := io.ReadAll(tr)
		if err != nil {
			return fnerrors.BadInputError("failed to read contents of %q: %v", h.Name, err)
		}

		if err := visitor(h.Name, b, tarDirent{h.FileInfo()}); err != nil {
			return err
		}
	}

	return nil
}

type HasHasString interface {
	Has(string) bool
}

func ReadFilesInto(fsys fnfs.WriteFS, tarStream TarStreamFunc, include HasHasString) ([]string, error) {
	f, err := tarStream()
	if err != nil {
		return nil, err
	}

	defer f.Close()

	var readFiles []string

	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fnerrors.BadInputError("unexpected error: %v", err)
		}

		clean := filepath.Clean(h.Name)

		if include.Has(clean) {
			w, err := fsys.OpenWrite(clean, h.FileInfo().Mode())
			if err != nil {
				return nil, fnerrors.TransientError("failed to open %q for writing (was %q)", clean, h.Name)
			}

			_, err = io.Copy(w, tr)
			w.Close() // Close
			if err != nil {
				return nil, fnerrors.TransientError("failed to copy %q (was %q): %v", clean, h.Name, err)
			}

			readFiles = append(readFiles, clean)
		}
	}

	return readFiles, nil
}
