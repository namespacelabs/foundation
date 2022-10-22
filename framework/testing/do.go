// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/schema"
)

var (
	testTimeout = flag.Duration("test_timeout", 5*time.Minute, "The maximum duration of the test.")
	debug       = flag.Bool("debug", false, "Output additional test runtime information.")
)

type Test struct {
	testboot.TestData
}

func (t Test) Connect(ctx context.Context, endpoint *schema.Endpoint) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, endpoint.Address(),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials())) ///  XXX mTLS etc.
}

func (t Test) WaitForEndpoint(ctx context.Context, endpoint *schema.Endpoint) error {
	ctx, done := context.WithTimeout(ctx, 10*time.Second)
	defer done()

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("endpoint not ready: %v", err)
		}

		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", endpoint.Address())
		if err == nil {
			conn.Close()
			return nil
		}
	}
}

func (t Test) MustEndpoint(owner, name string) *schema.Endpoint {
	for _, endpoint := range t.Request.Endpoint {
		if endpoint.EndpointOwner == owner && endpoint.ServiceName == name {
			return endpoint
		}
	}

	log.Fatalf("Expected endpoint to be present in the stack: endpoint_owner=%q service_name=%q", owner, name)
	return nil
}

func (t Test) InternalOf(serverOwner string) []*schema.InternalEndpoint {
	var filtered []*schema.InternalEndpoint
	for _, ie := range t.Request.InternalEndpoint {
		if ie.ServerOwner == serverOwner {
			filtered = append(filtered, ie)
		}
	}
	return filtered
}

func Do(testFunc func(context.Context, Test) error) {
	t := testboot.BootstrapTest(*testTimeout, *debug)

	if err := testFunc(context.Background(), Test{t}); err != nil {
		log.Fatal(err)
	}
}
