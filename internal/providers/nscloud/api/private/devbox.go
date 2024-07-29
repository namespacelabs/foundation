// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package private

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	devbox "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/private/devbox/devboxv1betagrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/public"
)

type DevBoxServiceClient struct {
	devbox.DevBoxServiceClient
}

func MakeDevBoxClient(ctx context.Context, token fnapi.ResolvedToken) (*DevBoxServiceClient, error) {
	rawEndpoint, err := fnapi.ResolveGlobalEndpoint(ctx, token)
	if err != nil {
		return nil, err
	}

	parsedEP, err := url.Parse(rawEndpoint)
	if err != nil {
		return nil, err
	}

	serverName := parsedEP.Hostname()
	endpoint := fmt.Sprintf("%s:443", serverName)

	fmt.Fprintf(console.Debug(ctx), "RPC: connecting to devbox service (endpoint: %s)\n", endpoint)

	conn, err := grpc.NewClient(endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})), public.WithBearerPerRPC(token.BearerToken))
	if err != nil {
		return nil, err
	}

	cli := devbox.NewDevBoxServiceClient(conn)
	return &DevBoxServiceClient{cli}, nil
}
