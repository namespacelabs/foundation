// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func DescribeCluster(ctx context.Context, devHost *schema.DevHost, env *schema.Environment, name string) (out *eks.DescribeClusterOutput, _ error) {
	return tasks.Return(ctx, tasks.Action("eks.describe-cluster").Category("aws"), func(ctx context.Context) (*eks.DescribeClusterOutput, error) {
		sesh, _, err := awsprovider.ConfiguredSession(ctx, devHost, env)
		if err != nil {
			return nil, err
		}

		out, err := eks.NewFromConfig(sesh).DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: &name,
		})
		if err != nil {
			return nil, err
		}

		if out.Cluster == nil {
			return nil, fnerrors.InvocationError("api didn't return a cluster description as expected")
		}

		return out, nil
	})
}
