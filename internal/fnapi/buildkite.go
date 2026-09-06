// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	"namespacelabs.dev/integrations/proto/namespace/cloud/buildkite/buildkiteconnect"
)

func NewBuildkiteJobsServiceClient(ctx context.Context) (buildkiteconnect.JobsServiceClient, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return buildkiteconnect.NewJobsServiceClient(
		http.DefaultClient,
		GlobalEndpoint(),
		connect.WithInterceptors(newAuthInterceptor(tok)),
	), nil
}
