// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package compute

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fntypes"
	"namespacelabs.dev/foundation/workspace/cache"
)

func RegisterBytesCacheable() {
	RegisterCacheable[[]byte](bytesCacheable{})
	RegisterCacheable[ByteStream](byteStreamCacheable{})
}

type bytesCacheable struct{}

func (bc bytesCacheable) ComputeDigest(_ context.Context, v interface{}) (fntypes.Digest, error) {
	return cache.DigestBytes(v.([]byte))
}

func (bc bytesCacheable) LoadCached(ctx context.Context, c cache.Cache, t reflect.Type, d fntypes.Digest) (Result[[]byte], error) {
	bytes, err := c.Bytes(ctx, d)
	if err != nil {
		return Result[[]byte]{}, err
	}

	return Result[[]byte]{Digest: d, Value: bytes}, nil
}

func (bc bytesCacheable) Cache(ctx context.Context, c cache.Cache, contents []byte) (fntypes.Digest, error) {
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

func (bc byteStreamCacheable) ComputeDigest(_ context.Context, v interface{}) (fntypes.Digest, error) {
	return v.(ByteStream).Digest(), nil
}

func (bc byteStreamCacheable) LoadCached(ctx context.Context, c cache.Cache, t reflect.Type, d fntypes.Digest) (Result[ByteStream], error) {
	f, err := c.Blob(d)
	if err != nil {
		return Result[ByteStream]{}, err
	}

	defer f.Close()

	if file, ok := f.(*os.File); ok {
		// Need to stat.
		if st, err := file.Stat(); err != nil {
			return Result[ByteStream]{}, fnerrors.InternalError("couldn't get content length of cache entry: %w", err)
		} else {
			return Result[ByteStream]{Digest: d, Value: cachedByteStream{c, d, uint64(st.Size())}}, nil
		}
	}

	return Result[ByteStream]{}, errors.New("unexpected cache implementation, couldn't get content length of cache entry")
}

func (bc byteStreamCacheable) Cache(ctx context.Context, c cache.Cache, v ByteStream) (fntypes.Digest, error) {
	if cached, ok := v.(cachedByteStream); ok {
		// Don't rewrite, it's already cached.
		return cached.digest, nil
	}

	f, err := v.Reader()
	if err != nil {
		return fntypes.Digest{}, err
	}

	return v.Digest(), c.WriteBlob(ctx, v.Digest(), f)
}

type cachedByteStream struct {
	cache         cache.Cache
	digest        fntypes.Digest
	contentLength uint64
}

var _ ByteStream = cachedByteStream{}

func (bs cachedByteStream) Digest() fntypes.Digest { return bs.digest }
func (bs cachedByteStream) ContentLength() uint64  { return bs.contentLength }
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