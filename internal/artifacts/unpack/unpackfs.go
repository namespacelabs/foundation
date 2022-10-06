// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package unpack

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Unpacked struct {
	Files string // Points to the unpacked filesystem.
}

func Unpack(what string, fsys compute.Computable[fs.FS], opts ...UnpackOpt) compute.Computable[Unpacked] {
	u := &unpackFS{what: what, fsys: fsys}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

// Rather than checksum all files, checksum only the specified ones.
func WithChecksumPaths(paths ...string) UnpackOpt {
	return func(uf *unpackFS) {
		uf.checksumPaths = append(uf.checksumPaths, paths...)
	}
}

func SkipChecksumCheck() UnpackOpt {
	return func(uf *unpackFS) {
		uf.skipChecksums = true
	}
}

type UnpackOpt func(*unpackFS)

type unpackFS struct {
	what          string
	fsys          compute.Computable[fs.FS]
	checksumPaths []string
	skipChecksums bool

	compute.LocalScoped[Unpacked]
}

var _ compute.Computable[Unpacked] = &unpackFS{}

func (u *unpackFS) Action() *tasks.ActionEvent { return tasks.Action(fmt.Sprintf("unpack.%s", u.what)) }
func (u *unpackFS) Inputs() *compute.In {
	return compute.Inputs().Computable("fsys", u.fsys).Str("what", u.what)
}
func (u *unpackFS) Output() compute.Output {
	// This node is not cacheable as we always want to validate the contents of the resulting path.
	return compute.Output{NotCacheable: true}
}

func (u *unpackFS) Compute(ctx context.Context, deps compute.Resolved) (Unpacked, error) {
	fsysv, _ := compute.GetDep(deps, u.fsys, "fsys")

	dir, err := dirs.Ensure(dirs.UnpackCache())
	if err != nil {
		return Unpacked{}, err
	}

	baseDir := filepath.Join(dir, fsysv.Digest.Algorithm, fsysv.Digest.Hex)
	targetDir := filepath.Join(baseDir, u.what)
	targetChecksum := filepath.Join(baseDir, "checksums.json")

	tasks.Attachments(ctx).AddResult("dir", targetDir)

	if checksumsBytes, err := ioutil.ReadFile(targetChecksum); err == nil {
		// Target exists, verify contents.
		var checksums []checksumEntry
		if err := json.Unmarshal(checksumsBytes, &checksums); err == nil {
			// If unmarshal fails, we'll just remove and replace below.
			eg := executor.New(ctx, "unpack.check-digests")
			for _, cksum := range checksums {
				cksum := cksum // Close cksum.
				eg.Go(func(ctx context.Context) error {
					if u.skipChecksums {
						_, err := os.Stat(filepath.Join(targetDir, cksum.Path))
						return err
					}

					f, err := os.Open(filepath.Join(targetDir, cksum.Path))
					if err != nil {
						return err
					}
					defer f.Close()

					h := sha256.New()
					if _, err := io.Copy(h, f); err != nil {
						return err
					}
					if !cksum.Digest.Equals(schema.FromHash("sha256", h)) {
						// Use a regular error here as we never pass this to the user, and we can keep it cheap.
						return errors.New("digest doesn't match")
					}
					return nil
				})
			}

			if err := eg.Wait(); err == nil {
				return Unpacked{targetDir}, nil
			}
		}
	}

	if err := os.RemoveAll(baseDir); err != nil {
		if !os.IsNotExist(err) {
			return Unpacked{}, fnerrors.UserError(nil, "failed to remove existing unpack directory: %w", err)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return Unpacked{}, fnerrors.UserError(nil, "failed to create target unpack directory: %w", err)
	}

	var checksums []checksumEntry
	// XXX parallelism.
	if err := fnfs.VisitFiles(ctx, fsysv.Value, func(path string, blob bytestream.ByteStream, de fs.DirEntry) error {
		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(filepath.Join(targetDir, dir), 0755); err != nil {
				return fnerrors.UserError(nil, "failed to create %q: %w", dir, err)
			}
		}

		fi, err := de.Info()
		if err != nil {
			return err
		}

		contents, err := blob.Reader()
		if err != nil {
			return err
		}

		defer contents.Close()

		file, err := os.OpenFile(filepath.Join(targetDir, path), os.O_CREATE|os.O_WRONLY, fi.Mode())
		if err != nil {
			return err
		}

		checksum := len(u.checksumPaths) == 0 || slices.Contains(u.checksumPaths, path)

		var w io.Writer
		var h hash.Hash
		if checksum {
			h = sha256.New()
			w = io.MultiWriter(file, h)
		} else {
			w = file
		}

		_, writeErr := io.Copy(w, contents)
		closeErr := file.Close()

		if writeErr != nil {
			return writeErr
		} else if closeErr != nil {
			return closeErr
		}

		if h != nil {
			checksums = append(checksums, checksumEntry{Path: path, Digest: schema.FromHash("sha256", h)})
		}

		return nil
	}); err != nil {
		return Unpacked{}, err
	}

	serializedChecksums, err := json.Marshal(checksums)
	if err != nil {
		return Unpacked{}, fnerrors.InternalError("failed to serialize checksums: %w", err)
	}

	if err := ioutil.WriteFile(targetChecksum, serializedChecksums, 0444); err != nil {
		return Unpacked{}, fnerrors.InternalError("failed to write checksum file: %w", err)
	}

	return Unpacked{targetDir}, nil
}

type checksumEntry struct {
	Path   string        `json:"path"`
	Digest schema.Digest `json:"digest"`
}
