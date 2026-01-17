// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package wscontents

import (
	"context"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cespare/xxhash/v2"
	"github.com/moby/patternmatcher"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func SnapshotDirectory(ctx context.Context, absPath string, excludeMatcher *patternmatcher.PatternMatcher, digest bool) (*memfs.FS, error) {
	fsys, err := snapshotDirectory(ctx, absPath, excludeMatcher, digest)
	if err != nil {
		return nil, fnerrors.InternalError("snapshot failed: %v", err)
	}
	return fsys, nil
}

func snapshotDirectory(ctx context.Context, absPath string, excludeMatcher *patternmatcher.PatternMatcher, digest bool) (*memfs.FS, error) {
	if err := verifyDir(absPath); err != nil {
		return nil, err
	}

	var inmem memfs.FS
	if err := filepath.WalkDir(absPath, func(osPathname string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if excludeMatcher != nil {
			if matches, err := excludeMatcher.MatchesOrParentMatches(osPathname); err != nil {
				return err
			} else if matches {
				if de.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
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

func verifyDir(path string) error {
	if st, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return err
		}
		return fnerrors.Newf("%s: accessing the path failed: %v", path, err)
	} else if !st.IsDir() {
		return fnerrors.Newf("%s: expected it to be a directory", path)
	}

	return nil
}

func digestfile(contents io.Reader) ([]byte, error) {
	h := xxhash.New()
	_, err := io.Copy(h, contents)
	return []byte(hex.EncodeToString(h.Sum(nil))), err
}
