// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute/cache"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func RegisterBytesCacheable() {
	RegisterCacheable[[]byte](bytesCacheable{})
	RegisterCacheable[bytestream.ByteStream](byteStreamCacheable{})
}

type bytesCacheable struct{}

func (bc bytesCacheable) ComputeDigest(_ context.Context, v interface{}) (schema.Digest, error) {
	return cache.DigestBytes(v.([]byte))
}

func (bc bytesCacheable) LoadCached(ctx context.Context, c cache.Cache, t CacheableInstance, d schema.Digest) (Result[[]byte], error) {
	bytes, err := c.Bytes(ctx, d)
	if err != nil {
		return Result[[]byte]{}, err
	}

	return Result[[]byte]{Digest: d, Value: bytes}, nil
}

func (bc bytesCacheable) Cache(ctx context.Context, c cache.Cache, contents []byte) (schema.Digest, error) {
	h, err := cache.DigestBytes(contents)
	if err != nil {
		return h, err
	}
	if err := c.WriteBytes(ctx, h, contents); err != nil {
		return h, err
	}
	return h, nil
}

type byteStreamCacheable struct{}

func (bc byteStreamCacheable) ComputeDigest(ctx context.Context, v interface{}) (schema.Digest, error) {
	return bytestream.Digest(ctx, v.(bytestream.ByteStream))
}

func (bc byteStreamCacheable) LoadCached(ctx context.Context, c cache.Cache, t CacheableInstance, d schema.Digest) (Result[bytestream.ByteStream], error) {
	f, err := c.Blob(d)
	if err != nil {
		return Result[bytestream.ByteStream]{}, err
	}

	defer f.Close()

	if file, ok := f.(*os.File); ok {
		// Need to stat.
		if st, err := file.Stat(); err != nil {
			return Result[bytestream.ByteStream]{}, fnerrors.InternalError("couldn't get content length of cache entry: %w", err)
		} else {
			return Result[bytestream.ByteStream]{Digest: d, Value: cachedByteStream{c, d, uint64(st.Size())}}, nil
		}
	}

	return Result[bytestream.ByteStream]{}, fnerrors.New("unexpected cache implementation, couldn't get content length of cache entry")
}

func (bc byteStreamCacheable) Cache(ctx context.Context, c cache.Cache, v bytestream.ByteStream) (schema.Digest, error) {
	if cached, ok := v.(cachedByteStream); ok {
		// Don't rewrite, it's already cached.
		return cached.digest, nil
	}

	f, err := v.Reader()
	if err != nil {
		return schema.Digest{}, err
	}

	digest, err := bytestream.Digest(ctx, v)
	if err != nil {
		return digest, err
	}

	if err := c.WriteBlob(ctx, digest, f); err != nil {
		return digest, err
	}

	return digest, nil
}

type cachedByteStream struct {
	cache         cache.Cache
	digest        schema.Digest
	contentLength uint64
}

var _ bytestream.ByteStream = cachedByteStream{}

func (bs cachedByteStream) ComputeDigest(context.Context) (schema.Digest, error) {
	return bs.digest, nil
}
func (bs cachedByteStream) ContentLength() uint64 { return bs.contentLength }
func (bs cachedByteStream) Reader() (io.ReadCloser, error) {
	return bs.cache.Blob(bs.digest)
}

func (bs cachedByteStream) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"cached":        true,
		"digest":        bs.digest,
		"contentLength": bs.contentLength,
	})
}
