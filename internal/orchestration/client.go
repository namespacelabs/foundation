// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/schema"
)

func Deploy(ctx context.Context, plan *schema.DeployPlan) (string, error) {
	req := &proto.DeployRequest{
		Plan: plan,
	}

	// TODO!
	endpointAddress := "TODO"

	resp := &proto.DeployResponse{}
	err := fnapi.AnonymousCall(ctx, endpointAddress, "nsl.orchestration.service.proto.OrchestrationService/Deploy", req, fnapi.DecodeJSONResponse(resp))

	return resp.Id, err
}
