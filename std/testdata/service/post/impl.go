// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package post

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/grpc/peer"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/testdata/service/proto"
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

func (svc *Service) StreamingPost(req *proto.PostRequest, srv proto.PostService_StreamingPostServer) error {
	//use wait group to allow process to be concurrent
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(count int64) {
			defer wg.Done()

			//time sleep to simulate server process time
			time.Sleep(time.Duration(count) * time.Second)
			response := &proto.PostResponse{Id: ids.NewSortableID(), Response: "hello there: " + req.GetInput()}
			if err := srv.Send(response); err != nil {
				log.Printf("send error %v", err)
			}
			log.Printf("finishing request number : %d", count)
		}(int64(i))
	}

	wg.Wait()
	return nil
}
func WireService(ctx context.Context, srv server.Registrar, deps ServiceDeps) {
	svc := &Service{}
	proto.RegisterPostServiceServer(srv, svc)
}
