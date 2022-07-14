// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package rds

import (
	"context"

	awsrds "github.com/aws/aws-sdk-go-v2/service/rds"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, def *schema.SerializedInvocation, op *OpEnsureDBCluster) (*ops.HandleResult, error) {
		sesh, err := awsprovider.MustConfiguredSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
		if err != nil {
			return nil, err
		}

		input := &awsrds.CreateDBClusterInput{
			// TODO!
			DBClusterIdentifier: &op.DbClusterIdentifier,
			Engine:              &op.Engine,
		}

		rdscli := awsrds.NewFromConfig(sesh.Config())

		if _, err := rdscli.CreateDBCluster(ctx, input); err != nil {
			return nil, fnerrors.InvocationError("failed to create database cluster: %w", err)
		}

		return nil, nil
	})
}
