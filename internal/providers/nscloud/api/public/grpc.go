// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package public

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func SetGrpcBearer(ctx context.Context, bearer string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", bearer))
}

func WithBearerPerRPC(bearer string) grpc.DialOption {
	return grpc.WithPerRPCCredentials(staticToken{bearer})
}

type staticToken struct {
	token string
}

func (t staticToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (staticToken) RequireTransportSecurity() bool {
	return true
}
