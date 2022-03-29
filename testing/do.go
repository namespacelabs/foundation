// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/testing/testboot"
	"namespacelabs.dev/foundation/schema"
)

type Test struct {
	testboot.TestData
}

func (t Test) Connect(ctx context.Context, endpoint *schema.Endpoint) (*grpc.ClientConn, error) {
	return grpc.DialContext(ctx, fmt.Sprintf("%s:%d", endpoint.AllocatedName, endpoint.Port.ContainerPort),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials())) ///  XXX mTLS etc.
}

func Do(testFunc func(context.Context, Test) error) {
	t := testboot.BootstrapTest()

	if err := testFunc(context.Background(), Test{t}); err != nil {
		log.Fatal(err)
	}
}