// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/std/testdata/gogrpcservice"
)

func main() {
	if err := do(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func do(ctx context.Context) error {
	log.Println("dialing", os.Args[1])

	conn, err := grpc.DialContext(ctx, os.Args[1], grpc.WithInsecure())
	if err != nil {
		return err
	}

	resp, err := gogrpcservice.NewPostServiceClient(conn).Post(ctx, &gogrpcservice.PostRequest{Input: "hello world"})
	if err != nil {
		return err
	}

	log.Printf("received: %+v\n", resp)

	return nil
}