// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Register() {
	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster", prepareDescribeCluster)

	client.RegisterProvider("aws/eks", func(ctx context.Context, ck *devhost.ConfigKey) (*clientcmdapi.Config, error) {
		conf := &EKSCluster{}

		if !ck.Selector.Select(ck.DevHost).Get(conf) {
			return nil, fnerrors.BadInputError("eks provider configured, but missing EKSCluster")
		}

		s, err := NewSession(ctx, ck.DevHost, ck.Selector)
		if err != nil {
			return nil, fnerrors.InternalError("failed to create session: %w", err)
		}

		return KubeconfigWithBearerToken(ctx, s, conf.Name)
	})

	client.RegisterBearerTokenProvider("eks", func(ctx context.Context, ck *devhost.ConfigKey) (string, error) {
		conf := &EKSCluster{}

		if !ck.Selector.Select(ck.DevHost).Get(conf) {
			return "", fnerrors.BadInputError("eks bearer token provider configured, but missing EKSCluster")
		}

		s, err := NewSession(ctx, ck.DevHost, ck.Selector)
		if err != nil {
			return "", fnerrors.InternalError("failed to create session: %w", err)
		}

		token, err := ComputeToken(ctx, s, conf.Name)
		return token.Token, err
	})

	RegisterGraphHandlers()
}

func prepareDescribeCluster(ctx context.Context, env ops.Environment, se *schema.Stack_Entry) (*frontend.PrepareProps, error) {
	s, err := NewOptionalSession(ctx, env.DevHost(), devhost.ByEnvironment(env.Proto()))
	if err != nil {
		return nil, err
	}

	if s == nil {
		return nil, nil
	}

	eksCluster, err := PrepareClusterInfo(ctx, s)
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

func PrepareClusterInfo(ctx context.Context, s *Session) (*EKSCluster, error) {
	rt, err := kubernetes.New(ctx, s.devHost, s.selector)
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
	cluster, err := DescribeCluster(ctx, s, sysInfo.EksClusterName)
	if err != nil {
		return nil, err
	}

	eksCluster := &EKSCluster{
		Name: sysInfo.EksClusterName,
		Arn:  *cluster.Arn,
	}

	if cluster.Identity != nil && cluster.Identity.Oidc != nil {
		eksCluster.OidcIssuer = *cluster.Identity.Oidc.Issuer
	}

	return eksCluster, nil
}

func DescribeCluster(ctx context.Context, s *Session, name string) (*types.Cluster, error) {
	return tasks.Return(ctx, tasks.Action("eks.describe-cluster").Category("aws"), func(ctx context.Context) (*types.Cluster, error) {
		out, err := s.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: &name,
		})
		if err != nil {
			return nil, auth.CheckNeedsLoginOr(s.sesh, err, func(err error) error {
				return fnerrors.New("eks: describe cluster failed: %w", err)
			})
		}

		if out.Cluster == nil {
			return nil, fnerrors.InvocationError("api didn't return a cluster description as expected")
		}

		return out.Cluster, nil
	})
}
