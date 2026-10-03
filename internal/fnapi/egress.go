// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"namespacelabs.dev/integrations/proto/namespace/cloud/network/v1beta/networkv1betaconnect"
)

func NewEgressPolicyServiceClient(ctx context.Context) (networkv1betaconnect.EgressPolicyServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return networkv1betaconnect.NewEgressPolicyServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	), nil
}
