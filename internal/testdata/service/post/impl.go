// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package post

import (
	"context"
	"log"

	"google.golang.org/grpc/peer"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
	"namespacelabs.dev/go-ids"
)

type Service struct {
	proto.UnimplementedPostServiceServer
}

func (svc *Service) Post(ctx context.Context, req *proto.PostRequest) (*proto.PostResponse, error) {
	log.Printf("new request\n")

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

func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{}
	proto.RegisterPostServiceServer(srv, svc)
}
