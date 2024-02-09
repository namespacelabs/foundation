// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package public

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/url"

	builder "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/cloud/builder/v1beta/builderv1betagrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
)

func NewBuilderServiceClient(ctx context.Context, tid string, token fnapi.ResolvedToken) (builder.BuilderServiceClient, *grpc.ClientConn, error) {
	rawEndpoint, err := endpoint.ResolveRegionalEndpoint(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	parsedEP, err := url.Parse(rawEndpoint)
	if err != nil {
		log.Fatal(err)
	}

	serverName := parsedEP.Hostname()
	endpoint := fmt.Sprintf("%s:443", serverName)

	fmt.Fprintf(console.Debug(ctx), "[%s] RPC: connecting to builder service (endpoint: %s)\n", tid, endpoint)

	conn, err := grpc.DialContext(ctx, endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})), WithBearerPerRPC(token.BearerToken))
	if err != nil {
		return nil, nil, err
	}

	return builder.NewBuilderServiceClient(conn), conn, nil
}
