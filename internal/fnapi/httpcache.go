// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/integrations/httpcache/v1beta/httpcachev1betaconnect"
	"connectrpc.com/connect"
	"namespacelabs.dev/integrations/api"
)

func NewHttpCacheServiceClient(ctx context.Context) (httpcachev1betaconnect.HttpCacheServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return NewHttpCacheServiceClientWithToken(tok), nil
}

func NewHttpCacheServiceClientWithToken(tok api.TokenSource) httpcachev1betaconnect.HttpCacheServiceClient {
	return httpcachev1betaconnect.NewHttpCacheServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	)
}
