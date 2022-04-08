// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package post

import (
	"context"
	"log"

	"google.golang.org/grpc/peer"
	"namespacelabs.dev/foundation/std/go/grpc/server"
)

type Service struct {
}

func (svc *Service) Post(ctx context.Context, req *PostRequest) (*PostResponse, error) {
	log.Printf("new request\n")

	p, ok := peer.FromContext(ctx)
	if ok {
		log.Printf("from: %+v\n", p)
	} else {
		log.Printf("no peer?\n")
	}

	log.Printf("request: %+v\n", req)

	response := &PostResponse{Response: "hello there: " + req.GetInput()}
	log.Printf("will reply with: %+v\n", response)

	return response, nil
}

func WireService(ctx context.Context, srv *server.Grpc, deps *ServiceDeps) {
	svc := &Service{}
	RegisterPostServiceServer(srv, svc)
}
