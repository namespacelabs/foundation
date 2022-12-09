// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"time"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

const VerifyCacheWrites = false
const indexDir = "index"

type CacheInfo interface {
	Size() int64
}

type Cache interface {
	Bytes(context.Context, schema.Digest) ([]byte, error)
	Blob(schema.Digest) (ReaderAtCloser, error) // XXX No context is required here to make it simpler for Compressed() to call Blob().
	Stat(context.Context, schema.Digest) (CacheInfo, error)

	WriteBlob(context.Context, schema.Digest, io.ReadCloser) error
	WriteBytes(context.Context, schema.Digest, []byte) error

	LoadEntry(context.Context, schema.Digest) (CachedOutput, bool, error)
	StoreEntry(context.Context, []schema.Digest, CachedOutput) error
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.ReadCloser
}

type CachedOutput struct {
	Digest       schema.Digest
	Timestamp    time.Time
	CacheVersion int
	InputDigests map[string]string

	Debug struct {
		PackagePath, Typename string
	}
}

func Local() (Cache, error) {
	cacheDir, err := dirs.Cache()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(cacheDir, "blobs")

	if err := os.MkdirAll(filepath.Join(dir, "sha256"), 0700|os.ModeDir); err != nil {
		return nil, err
	}

	return &localCache{path: dir}, err
}

func Prune(ctx context.Context) error {
	return tasks.Action("cache.prune").Run(ctx, func(ctx context.Context) error {
		cacheDir, err := dirs.Cache()
		if err != nil {
			return err
		}

		return os.RemoveAll(cacheDir)
	})
}

type localCache struct {
	path string
}

func (c *localCache) blobPath(h schema.Digest) string {
	return filepath.Join(c.path, h.Algorithm, h.Hex)
}

func (c *localCache) Bytes(ctx context.Context, h schema.Digest) ([]byte, error) {
	if h.Algorithm == "" || h.Hex == "" {
		return nil, fnerrors.InternalError("digest not set")
	}
	return os.ReadFile(c.blobPath(h))
}

func (c *localCache) Stat(ctx context.Context, h schema.Digest) (CacheInfo, error) {
	return os.Stat(c.blobPath(h))
}

func (c *localCache) Blob(h schema.Digest) (ReaderAtCloser, error) {
	if h.Algorithm == "" || h.Hex == "" {
		return nil, fnerrors.InternalError("digest not set")
	}
	return os.Open(c.blobPath(h))
}

func (c *localCache) WriteBytes(ctx context.Context, h schema.Digest, contents []byte) error {
	return c.WriteBlob(ctx, h, io.NopCloser(bytes.NewReader(contents)))
}

func (c *localCache) WriteBlob(ctx context.Context, h schema.Digest, r io.ReadCloser) error {
	if VerifyCacheWrites {
		r = verifyReader{reader: r, expected: h, hash: sha256.New()}
	}

	file := c.blobPath(h)
	if fi, err := os.Stat(file); err == nil {
		if VerifyCacheWrites {
			f, err := os.Open(file)
			if err != nil {
				return fnerrors.InternalError("failed to verify cache: %w", err)
			}
			defer f.Close()
			hash := sha256.New()
			if _, err := io.Copy(hash, f); err != nil {
				return fnerrors.InternalError("failed to verify cache entry: %w", err)
			}
			return verifyHash(h, hash)
		}

		artifacts.MaybeUpdateSkippedBytes(r, fi.Size())
		return r.Close()
	}

	rid := ids.NewRandomBase32ID(8)

	w, err := os.Create(filepath.Join(c.path, h.Algorithm, fmt.Sprintf(".%s.%s", h.Hex, rid)))
	if err != nil {
		r.Close()
		return err
	}
	defer w.Close()

	_, copyErr := io.Copy(w, r)
	closeErr := r.Close()

	if copyErr != nil {
		return copyErr
	}

	if err := closeErr; err != nil {
		return closeErr
	}

	return os.Rename(w.Name(), c.blobPath(h))
}

type verifyReader struct {
	reader   io.ReadCloser
	expected schema.Digest
	hash     hash.Hash
}

func (vr verifyReader) Read(p []byte) (int, error) {
	n, err := vr.reader.Read(p)
	vr.hash.Write(p[:n])
	return n, err
}

func (vr verifyReader) Close() error {
	if err := vr.reader.Close(); err != nil {
		return err
	}

	return verifyHash(vr.expected, vr.hash)
}

func verifyHash(expected schema.Digest, hash hash.Hash) error {
	got := schema.FromHash("sha256", hash)
	if got != expected {
		return fnerrors.InternalError("digest didn't match, expected %q got %q", expected.String(), got.String())
	}
	return nil
}

func DigestBytes(contents []byte) (schema.Digest, error) {
	h := sha256.New()
	if _, err := h.Write(contents); err != nil {
		return schema.Digest{}, nil
	}
	return schema.Digest{Algorithm: "sha256", Hex: hex.EncodeToString(h.Sum(nil))}, nil
}

func (c *localCache) LoadEntry(ctx context.Context, h schema.Digest) (CachedOutput, bool, error) {
	indexFile := filepath.Join(c.path, indexDir, h.String()+".json")
	f, err := os.Open(indexFile)
	if err != nil {
		if os.IsNotExist(err) {
			return CachedOutput{}, false, nil
		}
		return CachedOutput{}, false, err
	}

	defer f.Close()

	var out CachedOutput
	if err := json.NewDecoder(f).Decode(&out); err != nil {
		return out, false, fnerrors.InternalError("failed to decode cached entry: %w", err)
	}

	return out, true, nil
}

func (c *localCache) StoreEntry(ctx context.Context, inputs []schema.Digest, output CachedOutput) error {
	if len(inputs) == 0 {
		return nil
	}

	indexDir := filepath.Join(c.path, indexDir)
	if err := os.MkdirAll(indexDir, 0700); err != nil {
		return fnerrors.InternalError("failed to create cache index dir: %w", err)
	}

	for _, input := range inputs {
		if !input.IsSet() {
			continue
		}

		indexFile := filepath.Join(indexDir, input.String()+".json")
		t, err := os.ReadFile(indexFile)
		if err == nil {
			var existing CachedOutput
			if err := json.Unmarshal(t, &existing); err == nil {
				if !existing.Digest.Equals(output.Digest) {
					fmt.Fprintf(console.Warnings(ctx), "cache.StoreEntry: non-determinism, overwriting pointer; input=%s existing=%v output=%v\n", input, existing.Digest, output.Digest)
				}
			}
		}

		marshalled, err := json.Marshal(output)
		if err != nil {
			return fnerrors.InternalError("failed to marshal cached output: %w", err)
		}

		tmpid := ids.NewRandomBase32ID(4)

		tmpFile := indexFile + "." + tmpid
		if err := os.WriteFile(tmpFile, marshalled, 0600); err != nil {
			return fnerrors.InternalError("failed to write cached output: %w", err)
		}

		if err := os.Rename(tmpFile, indexFile); err != nil {
			return fnerrors.InternalError("failed to commit cached output: %w", err)
		}
	}

	return nil
}
