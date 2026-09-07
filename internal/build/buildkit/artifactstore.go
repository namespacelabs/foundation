// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/errdefs"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/artifacts/contentstore"
	"namespacelabs.dev/foundation/schema"
)

type artifactStore struct {
	store contentstore.Store
}

var _ content.Store = &artifactStore{}

func (s *artifactStore) Info(ctx context.Context, dgst digest.Digest) (content.Info, error) {
	d, err := schema.ParseDigest(dgst.String())
	if err != nil {
		return content.Info{}, err
	}

	info, err := s.store.Stat(ctx, d)
	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("content %v: %w", dgst, errdefs.ErrNotFound)
		}
		return content.Info{}, err
	}

	return content.Info{
		Digest: dgst,
		Size:   info.Size(),
	}, nil
}

func (s *artifactStore) Update(ctx context.Context, info content.Info, fieldpaths ...string) (content.Info, error) {
	return content.Info{}, rpcerrors.Errorf(codes.Unimplemented, "update: writes not supported")
}

func (s *artifactStore) Walk(ctx context.Context, fn content.WalkFunc, filters ...string) error {
	return rpcerrors.Errorf(codes.Unimplemented, "walk: not implemented")
}

// Delete removes the content from the store.
func (s *artifactStore) Delete(ctx context.Context, dgst digest.Digest) error {
	return rpcerrors.Errorf(codes.Unimplemented, "delete: writes not supported")
}

func (s *artifactStore) ReaderAt(ctx context.Context, desc ocispec.Descriptor) (content.ReaderAt, error) {
	d, err := schema.ParseDigest(desc.Digest.String())
	if err != nil {
		return nil, err
	}

	info, err := s.store.Stat(ctx, d)
	if err != nil {
		return nil, err
	}

	r, err := s.store.Blob(d)
	if err != nil {
		return nil, err
	}

	return storeReader{r, info.Size()}, nil
}

func (s *artifactStore) Status(ctx context.Context, ref string) (content.Status, error) {
	return content.Status{}, rpcerrors.Errorf(codes.Unimplemented, "status: not implemented")
}

func (s *artifactStore) ListStatuses(ctx context.Context, filters ...string) ([]content.Status, error) {
	return nil, rpcerrors.Errorf(codes.Unimplemented, "liststatuses: not implemented")
}

func (s *artifactStore) Abort(ctx context.Context, ref string) error {
	return rpcerrors.Errorf(codes.Unimplemented, "abort: not implemented")
}

func (s *artifactStore) Writer(ctx context.Context, opts ...content.WriterOpt) (content.Writer, error) {
	return nil, rpcerrors.Errorf(codes.Unimplemented, "writer: writes not supported")
}

type storeReader struct {
	contentstore.ReaderAtCloser
	size int64
}

func (s storeReader) Size() int64 { return s.size }
