// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package minio

import (
	"bytes"
	"context"
	"io"
	"strings"

	"google.golang.org/protobuf/types/known/emptypb"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/universe/aws/s3"
)

type Service struct {
	bucket *s3.Bucket
}

func convToString(r io.ReadCloser) string {
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	return buf.String()
}

func (s *Service) Add(ctx context.Context, req *AddRequest) (*emptypb.Empty, error) {
	_, err := s.bucket.PutObject(ctx,
		req.Filename,
		strings.NewReader(req.Contents))
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *Service) Get(ctx context.Context, req *GetRequest) (*GetResponse, error) {
	out, err := s.bucket.GetObject(ctx, req.Filename)
	if err != nil {
		return nil, err
	}

	return &GetResponse{Contents: convToString(out.Body)}, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{bucket: deps.Bucket}
	RegisterS3DemoServiceServer(srv, svc)
}
