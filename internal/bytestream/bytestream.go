// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package bytestream

import (
	"context"
	"crypto/sha256"
	"io"

	"namespacelabs.dev/foundation/schema"
)

type ByteStream interface {
	ContentLength() uint64
	Reader() (io.ReadCloser, error)
}

type ByteStreamWithDigest interface {
	ByteStream
	ComputeDigest(context.Context) (schema.Digest, error)
}

func Digest(ctx context.Context, bs ByteStream) (schema.Digest, error) {
	if cd, ok := bs.(interface {
		ComputeDigest(context.Context) (schema.Digest, error)
	}); ok {
		return cd.ComputeDigest(ctx)
	}

	r, err := bs.Reader()
	if err != nil {
		return schema.Digest{}, err
	}

	defer r.Close()

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return schema.Digest{}, err
	}

	return schema.FromHash("sha256", h), nil
}
