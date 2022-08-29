// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/schema"
)

func Deploy(ctx context.Context, plan *schema.DeployPlan) (string, error) {
	req := &proto.DeployRequest{
		Plan: plan,
	}

	// TODO!
	endpointAddress := "TODO"
	conn, err := grpc.DialContext(ctx, endpointAddress)
	if err != nil {
		return "", err
	}

	cli := proto.NewOrchestrationServiceClient(conn)

	resp, err := cli.Deploy(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Id, nil
}
