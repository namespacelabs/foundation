// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/integrations/bazel/v1beta/bazelv1betaconnect"
	"connectrpc.com/connect"
)

func NewBazelCacheServiceClient(ctx context.Context) (bazelv1betaconnect.BazelCacheServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	client := bazelv1betaconnect.NewBazelCacheServiceClient(
		http.DefaultClient,
		"https://global.namespaceapis.com",
		connect.WithInterceptors(newAuthInterceptor(tok)),
	)

	return client, nil
}
