// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package post

import (
	"context"
	"log"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/go-ids"
)

type Service struct {
	proto.UnimplementedPostServiceServer
}

func (svc *Service) Post(ctx context.Context, req *proto.PostRequest) (*proto.PostResponse, error) {
	zerolog.Ctx(ctx).Info().Msg("new Post request")

	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("from: %+v\n", p)
	} else {
		log.Printf("no peer?\n")
	}

	log.Printf("request: %+v\n", req)

	response := &proto.PostResponse{Id: ids.NewSortableID(), Response: "hello there: " + req.GetInput()}
	log.Printf("will reply with: %+v\n", response)

	return response, nil
}

func (svc *Service) TestTranscoding(ctx context.Context, req *emptypb.Empty) (*proto.TestTranscodingResponse, error) {
	zerolog.Ctx(ctx).Info().Msg("new TestTranscoding request")

	res := &proto.FetchResponse{
		Response: "foo",
	}

	any, err := anypb.New(res)
	if err != nil {
		return nil, err
	}

	return &proto.TestTranscodingResponse{
		Any:       any,
		Timestamp: timestamppb.Now(),
		Duration:  durationpb.New(time.Hour),
	}, nil
}

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{}
	proto.RegisterPostServiceServer(srv, svc)
}
