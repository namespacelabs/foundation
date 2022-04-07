// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/go-ids"
)

const VerifyCacheWrites = false
const mapJson = "map.json"

type Cache interface {
	Bytes(context.Context, schema.Digest) ([]byte, error)
	Blob(schema.Digest) (io.ReadCloser, error) // XXX No context is required here to make it simpler for Compressed() to call Blob().

	WriteBlob(context.Context, schema.Digest, io.ReadCloser) error
	WriteBytes(context.Context, schema.Digest, []byte) error

	LoadEntry(context.Context, schema.Digest) (CachedOutput, bool, error)
	StoreEntry(context.Context, []schema.Digest, CachedOutput) error
}

type CachedOutput struct {
	Digest       schema.Digest
	Timestamp    time.Time
	CacheVersion int
	InputDigests map[string]string

	Debug struct {
		Serial                int64
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

type cacheIndex struct {
	Outputs map[string]CachedOutput
}

func (c *localCache) blobPath(h schema.Digest) string {
	return filepath.Join(c.path, h.Algorithm, h.Hex)
}

func (c *localCache) Bytes(ctx context.Context, h schema.Digest) ([]byte, error) {
	if h.Algorithm == "" || h.Hex == "" {
		return nil, fnerrors.InternalError("digest not set")
	}
	return ioutil.ReadFile(c.blobPath(h))
}

func (c *localCache) Blob(h schema.Digest) (io.ReadCloser, error) {
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
	if _, err := os.Stat(file); err == nil {
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
		return nil
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
	index, err := loadIndex(ctx, string(c.path))
	if err != nil {
		return CachedOutput{}, false, err
	}

	output, ok := index.Outputs[h.String()]
	return output, ok, nil
}

func (c *localCache) StoreEntry(ctx context.Context, inputs []schema.Digest, output CachedOutput) error {
	if len(inputs) == 0 {
		return nil
	}

	// XXX consider using a model where each index entry is a separate file. That would
	// reduce the number of updates that need to happen here, and would reduce raciness
	// with updating the index. Another option would be to use something like sqlite, but
	// that would incur a building + maintenance cost on `fn`.

	index, err := loadIndex(ctx, string(c.path))
	if err != nil {
		return err
	}

	for _, input := range inputs {
		if !input.IsSet() {
			continue
		}

		if existing, ok := index.Outputs[input.String()]; ok && existing.Digest != output.Digest {
			fmt.Fprintf(console.Warnings(ctx), "cache.StoreEntry: non-determinism, overwriting pointer; input=%s existing=%v output=%v\n", input, existing.Digest, output.Digest)
		}

		index.Outputs[input.String()] = output
	}

	f, err := os.CreateTemp(string(c.path), mapJson)
	if err != nil {
		return err
	}

	err = json.NewEncoder(f).Encode(index)
	f.Close()

	if err != nil {
		return err
	}

	return os.Rename(f.Name(), filepath.Join(string(c.path), mapJson))
}

func loadIndex(ctx context.Context, path string) (cacheIndex, error) {
	contents, err := ioutil.ReadFile(filepath.Join(path, mapJson))
	if err != nil {
		if os.IsNotExist(err) {
			return cacheIndex{Outputs: map[string]CachedOutput{}}, nil
		}
		return cacheIndex{}, nil
	}

	var index cacheIndex
	if err := json.Unmarshal(contents, &index); err != nil {
		index = cacheIndex{}

		fmt.Fprintf(console.Warnings(ctx), "Dropped existing index, invalid format. Got: %v\n", err)
	}

	if index.Outputs == nil {
		index.Outputs = map[string]CachedOutput{}
	}

	return index, nil
}
