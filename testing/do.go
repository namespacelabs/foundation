// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/schema"
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

func Do(testFunc func(context.Context, Test) error) {
	t := testboot.BootstrapTest()

	if err := testFunc(context.Background(), Test{t}); err != nil {
		log.Fatal(err)
	}
}
