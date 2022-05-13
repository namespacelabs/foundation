// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package bytestream

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"io/ioutil"

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

type Static struct {
	Contents []byte
}

func (s Static) ContentLength() uint64 { return uint64(len(s.Contents)) }
func (s Static) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.Contents)), nil
}

func WriteTo(w io.Writer, contents ByteStream) error {
	r, err := contents.Reader()
	if err != nil {
		return err
	}

	defer r.Close()

	if _, err := io.Copy(w, r); err != nil {
		return err
	}

	return nil
}

func ReadAll(contents ByteStream) ([]byte, error) {
	r, err := contents.Reader()
	if err != nil {
		return nil, err
	}

	defer r.Close()

	return ioutil.ReadAll(r)
}
