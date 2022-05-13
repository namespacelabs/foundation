// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type ReadWriteFS interface {
	fs.ReadDirFS
	WriteFS
}

func WriteWorkspaceFile(ctx context.Context, log io.Writer, vfs ReadWriteFS, filePath string, h func(io.Writer) error) error {
	return WriteFileExtended(ctx, vfs, filePath, 0644, WriteFileExtendedOpts{
		CompareContents: true,
		AnnounceWrite:   log,
		EnsureFileMode:  false,
	}, h)
}

func WriteFSToWorkspace(ctx context.Context, log io.Writer, vfs ReadWriteFS, src fs.FS) error {
	return VisitFiles(ctx, src, func(path string, contents bytestream.ByteStream, dirent fs.DirEntry) error {
		return WriteWorkspaceFile(ctx, log, vfs, path, func(w io.Writer) error {
			return bytestream.WriteTo(w, contents)
		})
	})
}

type WriteFileExtendedOpts struct {
	ContentsDigest  schema.Digest
	CompareContents bool
	FailOverwrite   bool
	EnsureFileMode  bool
	AnnounceWrite   io.Writer
}

func WriteFileExtended(ctx context.Context, dst ReadWriteFS, filePath string, mode fs.FileMode, opts WriteFileExtendedOpts, writeContents func(io.Writer) error) error {
	if opts.ContentsDigest.IsSet() || opts.CompareContents {
		f, err := dst.Open(filePath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				goto write
			}
			return fnerrors.UserError(nil, "failed to write a workspace file %q: %w", filePath, err)
		}

		defer f.Close()

		if opts.CompareContents {
			// XXX use a compare writer, instead of buffering contents.
			contents, err := ioutil.ReadAll(f)
			if err != nil {
				return err
			}

			var b bytes.Buffer
			if err := writeContents(&b); err != nil {
				return err
			}

			if bytes.Equal(contents, b.Bytes()) {
				// Nothing to do.
				if opts.EnsureFileMode {
					return chmod(dst, filePath, mode)
				}
				return nil
			}
		} else {
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				return err
			}

			if schema.FromHash("sha256", h) == opts.ContentsDigest {
				// Contents are good.
				if opts.EnsureFileMode {
					return chmod(dst, filePath, mode)
				}
				return nil
			}
		}
	}

write:
	if opts.FailOverwrite {
		return fmt.Errorf("%s: would have been rewritten", filePath)
	}

	if mkfs, ok := dst.(MkdirFS); ok {
		if err := mkfs.MkdirAll(filepath.Dir(filePath), addExecToRead(mode)); err != nil {
			return err
		}
	}

	f, err := dst.OpenWrite(filePath, mode)
	if err != nil {
		return err
	}

	err = writeContents(f)
	if err1 := f.Close(); err1 != nil && err == nil {
		err = err1
	}

	if err != nil {
		return err
	}

	if opts.AnnounceWrite != nil {
		fmt.Fprintf(opts.AnnounceWrite, "Wrote %s\n", filePath)
	}

	if opts.EnsureFileMode {
		return chmod(dst, filePath, mode)
	}

	return nil
}

func chmod(fsys fs.FS, filePath string, mode fs.FileMode) error {
	if chm, ok := fsys.(ChmodFS); ok {
		if st, err := fs.Stat(fsys, filePath); err != nil {
			return err
		} else if st.Mode().Perm() == mode.Perm() {
			return nil
		}

		return chm.Chmod(filePath, mode.Perm())
	}

	return nil
}
