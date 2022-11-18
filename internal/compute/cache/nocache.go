// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cache

import (
	"context"
	"io"
	"os"

	"namespacelabs.dev/foundation/schema"
)

var NoCache Cache = noCache{}

type noCache struct{}

func (noCache) Bytes(context.Context, schema.Digest) ([]byte, error)   { return nil, os.ErrNotExist }
func (noCache) Blob(schema.Digest) (io.ReadCloser, error)              { return nil, os.ErrNotExist }
func (noCache) Stat(context.Context, schema.Digest) (CacheInfo, error) { return nil, os.ErrNotExist }

func (noCache) WriteBlob(context.Context, schema.Digest, io.ReadCloser) error { return nil }
func (noCache) WriteBytes(context.Context, schema.Digest, []byte) error       { return nil }

func (noCache) LoadEntry(context.Context, schema.Digest) (CachedOutput, bool, error) {
	return CachedOutput{}, false, nil
}
func (noCache) StoreEntry(context.Context, []schema.Digest, CachedOutput) error { return nil }

func (noCache) isDisabled() bool { return true }

func IsDisabled(cache Cache) bool {
	if impl, ok := cache.(interface {
		isDisabled() bool
	}); ok {
		return impl.isDisabled()
	}
	return false
}
