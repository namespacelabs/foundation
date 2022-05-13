// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package unpack

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var CheckChecksums = true

func Unpack(fsys compute.Computable[fs.FS]) compute.Computable[string] {
	return &unpackFS{fsys: fsys}
}

type unpackFS struct {
	fsys compute.Computable[fs.FS]

	compute.LocalScoped[string]
}

var _ compute.Computable[string] = &unpackFS{}

func (u *unpackFS) Action() *tasks.ActionEvent { return tasks.Action("fs.unpack") }
func (u *unpackFS) Inputs() *compute.In {
	return compute.Inputs().Computable("fsys", u.fsys)
}
func (u *unpackFS) Output() compute.Output {
	// This node is not cacheable as we always want to validate the contents of the resulting path.
	return compute.Output{NotCacheable: true}
}

func (u *unpackFS) Compute(ctx context.Context, deps compute.Resolved) (string, error) {
	fsysv, _ := compute.GetDep(deps, u.fsys, "fsys")

	dir, err := dirs.Ensure(dirs.UnpackCache())
	if err != nil {
		return "", err
	}

	baseDir := filepath.Join(dir, fsysv.Digest.Algorithm, fsysv.Digest.Hex)
	targetDir := filepath.Join(baseDir, "files")
	targetChecksum := filepath.Join(baseDir, "checksums.json")

	tasks.Attachments(ctx).AddResult("dir", targetDir)

	if checksumsBytes, err := ioutil.ReadFile(targetChecksum); err == nil {
		// Target exists, verify contents.
		var checksums []checksumEntry
		if err := json.Unmarshal(checksumsBytes, &checksums); err == nil {
			// If unmarshal fails, we'll just remove and replace below.

			ex, wait := executor.New(ctx)
			for _, cksum := range checksums {
				cksum := cksum // Close cksum.
				ex.Go(func(ctx context.Context) error {
					if !CheckChecksums {
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

			if err := wait(); err == nil {
				return targetDir, nil
			}
		}
	}

	if err := os.RemoveAll(baseDir); err != nil {
		if !os.IsNotExist(err) {
			return "", fnerrors.UserError(nil, "failed to remove existing unpack directory: %w", err)
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fnerrors.UserError(nil, "failed to create target unpack directory: %w", err)
	}

	var checksums []checksumEntry
	// XXX parallelism.
	if err := fnfs.VisitFiles(ctx, fsysv.Value, func(path string, contents []byte, de fs.DirEntry) error {
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

		if err := ioutil.WriteFile(filepath.Join(targetDir, path), contents, fi.Mode()); err != nil {
			return err
		}

		h := sha256.New()
		h.Write(contents)
		checksums = append(checksums, checksumEntry{Path: path, Digest: schema.FromHash("sha256", h)})
		return nil
	}); err != nil {
		return "", err
	}

	serializedChecksums, err := json.Marshal(checksums)
	if err != nil {
		return "", fnerrors.InternalError("failed to serialize checksums: %w", err)
	}

	if err := ioutil.WriteFile(targetChecksum, serializedChecksums, 0444); err != nil {
		return "", fnerrors.InternalError("failed to write checksum file: %w", err)
	}

	return targetDir, nil
}

type checksumEntry struct {
	Path   string        `json:"path"`
	Digest schema.Digest `json:"digest"`
}
