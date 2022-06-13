// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Register() {
	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster", prepareDescribeCluster)

	client.RegisterBearerTokenProvider("eks", func(ctx context.Context, ck *devhost.ConfigKey) (string, error) {
		conf := &EKSCluster{}

		if !ck.Selector.Select(ck.DevHost).Get(conf) {
			return "", fnerrors.BadInputError("eks bearer token provider configured, but missing EKSCluster")
		}

		token, _, err := ComputeToken(ctx, ck.DevHost, ck.Selector, conf.Name)
		return token.Token, err
	})
}

func prepareDescribeCluster(ctx context.Context, env ops.Environment, se *schema.Stack_Entry) (*frontend.PrepareProps, error) {
	eksCluster, err := PrepareClusterInfo(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
	if err != nil {
		return nil, err
	}

	if eksCluster == nil {
		return nil, nil
	}

	srv := se.Server
	eksServerDetails := &EKSServerDetails{
		ComputedIamRoleName: fmt.Sprintf("fn-%s-%s-%s", eksCluster.Name, env.Proto().Name, srv.Id),
	}

	if len(eksServerDetails.ComputedIamRoleName) > 64 {
		return nil, fnerrors.InternalError("generated a role name that is too long (%d): %s",
			len(eksServerDetails.ComputedIamRoleName), eksServerDetails.ComputedIamRoleName)
	}

	props := &frontend.PrepareProps{}

	if err := props.AppendInputs(eksCluster, eksServerDetails); err != nil {
		return nil, err
	}

	return props, nil
}

func PrepareClusterInfo(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (*EKSCluster, error) {
	rt, err := kubernetes.New(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	sysInfo, err := rt.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	if sysInfo.DetectedDistribution != "eks" || sysInfo.EksClusterName == "" {
		return nil, nil
	}

	// XXX use a compute.Computable here to cache the cluster information if multiple servers depend on it.
	description, err := DescribeCluster(ctx, devHost, selector, sysInfo.EksClusterName)
	if err != nil {
		return nil, err
	}

	eksCluster := &EKSCluster{
		Name: sysInfo.EksClusterName,
		Arn:  *description.Cluster.Arn,
	}

	if description.Cluster.Identity != nil && description.Cluster.Identity.Oidc != nil {
		eksCluster.OidcIssuer = *description.Cluster.Identity.Oidc.Issuer
	}

	return eksCluster, nil
}

func DescribeCluster(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector, name string) (out *eks.DescribeClusterOutput, _ error) {
	return tasks.Return(ctx, tasks.Action("eks.describe-cluster").Category("aws"), func(ctx context.Context) (*eks.DescribeClusterOutput, error) {
		sesh, _, err := awsprovider.ConfiguredSession(ctx, devHost, selector)
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
