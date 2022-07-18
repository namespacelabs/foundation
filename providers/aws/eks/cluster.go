// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const minimumTokenExpiry = 5 * time.Minute

func Register() {
	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster", prepareDescribeCluster)

	client.RegisterProvider("eks", provideEKS)
	client.RegisterProvider("aws/eks", provideEKS)

	RegisterGraphHandlers()
}

func provideEKS(ctx context.Context, ck *devhost.ConfigKey) (client.Provider, error) {
	conf := &EKSCluster{}

	if !ck.Selector.Select(ck.DevHost).Get(conf) {
		return client.Provider{}, fnerrors.BadInputError("eks provider configured, but missing EKSCluster")
	}

	s, err := NewSession(ctx, ck.DevHost, ck.Selector)
	if err != nil {
		return client.Provider{}, fnerrors.InternalError("failed to create session: %w", err)
	}

	cfg, err := KubeconfigFromCluster(ctx, s, conf.Name)
	if err != nil {
		return client.Provider{}, err
	}

	var mu sync.Mutex
	var lastToken *Token

	return client.Provider{
		Config: *cfg,
		TokenProvider: func(ctx context.Context) (string, error) {
			mu.Lock()
			l := lastToken
			mu.Unlock()

			if l != nil && time.Now().Add(minimumTokenExpiry).Before(l.Expiration) {
				return l.Token, nil
			}

			token, err := ComputeBearerToken(ctx, s, conf.Name)
			if err != nil {
				return "", err
			}

			mu.Lock()
			lastToken = &token
			mu.Unlock()

			return token.Token, nil
		},
	}, nil
}

func prepareDescribeCluster(ctx context.Context, env ops.Environment, se *schema.Stack_Entry) (*frontend.PrepareProps, error) {
	// XXX this breaks test/production similarity, but for the moment hide EKS
	// from tests. This removes the ability for tests to allocate IAM resources.
	if env.Proto().Ephemeral {
		return nil, nil
	}

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
		Name:  sysInfo.EksClusterName,
		Arn:   *cluster.Arn,
		VpcId: *cluster.ResourcesVpcConfig.VpcId,
	}

	if cluster.Identity != nil && cluster.Identity.Oidc != nil {
		eksCluster.OidcIssuer = *cluster.Identity.Oidc.Issuer
	}

	return eksCluster, nil
}

func DescribeCluster(ctx context.Context, s *Session, name string) (*types.Cluster, error) {
	return compute.GetValue[*types.Cluster](ctx, &cachedDescribeCluster{session: s, name: name})
}

type cachedDescribeCluster struct {
	session *Session
	name    string

	compute.DoScoped[*types.Cluster]
}

func (cd *cachedDescribeCluster) Action() *tasks.ActionEvent {
	return tasks.Action("eks.describe-cluster").Category("aws").Arg("name", cd.name)
}

func (cd *cachedDescribeCluster) Inputs() *compute.In {
	return compute.Inputs().Str("session", cd.session.sesh.CacheKey()).Str("name", cd.name)
}

func (cd *cachedDescribeCluster) Output() compute.Output { return compute.Output{NotCacheable: true} }

func (cd *cachedDescribeCluster) Compute(ctx context.Context, _ compute.Resolved) (*types.Cluster, error) {
	out, err := cd.session.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: &cd.name,
	})
	if err != nil {
		return nil, auth.CheckNeedsLoginOr(cd.session.sesh, err, func(err error) error {
			return fnerrors.New("eks: describe cluster failed: %w", err)
		})
	}

	if out.Cluster == nil {
		return nil, fnerrors.InvocationError("api didn't return a cluster description as expected")
	}

	return out.Cluster, nil
}
