// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/providers/aws/auth"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	eksconfig "namespacelabs.dev/foundation/universe/aws/configuration/eks"
	fneks "namespacelabs.dev/foundation/universe/aws/eks"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const minimumTokenExpiry = 5 * time.Minute

var clusterConfigType = planning.DefineConfigType[*fneks.EKSCluster]("foundation.providers.aws.eks.EKSCluster")

func Register() {
	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/universe/aws/eks.DescribeCluster", prepareDescribeCluster)

	client.RegisterConfigurationProvider("eks", provideEKS)
	client.RegisterConfigurationProvider("aws/eks", provideEKS)

	planning.RegisterConfigurationProvider(func(cluster *eksconfig.Cluster) ([]proto.Message, error) {
		if cluster.Name == "" {
			return nil, fnerrors.BadInputError("cluster name must be specified")
		}

		return []proto.Message{
			&client.HostEnv{Provider: "aws/eks"},
			&registry.Provider{Provider: "aws/ecr"},
			&fneks.EKSCluster{Name: cluster.Name},
		}, nil
	}, "foundation.providers.aws.eks.config.Cluster")

	RegisterGraphHandlers()
}

func provideEKS(ctx context.Context, config planning.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := clusterConfigType.CheckGet(config)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.BadInputError("eks provider configured, but missing EKSCluster")
	}

	s, err := NewSession(ctx, config)
	if err != nil {
		return client.ClusterConfiguration{}, fnerrors.InternalError("failed to create session: %w", err)
	}

	kubecfg, err := KubeconfigFromCluster(ctx, s, conf.Name)
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	var mu sync.Mutex
	var lastToken *Token

	return client.ClusterConfiguration{
		Config: *kubecfg,
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

func prepareDescribeCluster(ctx context.Context, env planning.Context, se *schema.Stack_Entry) (*frontend.PrepareProps, error) {
	// XXX this breaks test/production similarity, but for the moment hide EKS
	// from tests. This removes the ability for tests to allocate IAM resources.
	if env.Environment().Ephemeral {
		return nil, nil
	}

	s, err := NewOptionalSession(ctx, env.Configuration())
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
	eksServerDetails := &fneks.EKSServerDetails{
		ComputedIamRoleName: fmt.Sprintf("fn-%s-%s-%s", eksCluster.Name, env.Environment().Name, srv.Id),
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

func PrepareClusterInfo(ctx context.Context, s *Session) (*fneks.EKSCluster, error) {
	rt, err := kubernetes.ConnectToCluster(ctx, s.cfg)
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

	// XXX use a compute.Computable here to cache the awsCluster information if multiple servers depend on it.
	awsCluster, err := DescribeCluster(ctx, s, sysInfo.EksClusterName)
	if err != nil {
		return nil, err
	}
	cluster := awsCluster.Cluster

	eksCluster := &fneks.EKSCluster{
		Name:            sysInfo.EksClusterName,
		Arn:             *cluster.Arn,
		VpcId:           *cluster.ResourcesVpcConfig.VpcId,
		SecurityGroupId: *cluster.ResourcesVpcConfig.ClusterSecurityGroupId,
	}

	if cluster.Identity != nil && cluster.Identity.Oidc != nil {
		eksCluster.OidcIssuer = *cluster.Identity.Oidc.Issuer
	}
	eksCluster.HasOidcProvider = awsCluster.HasOidcProvider

	return eksCluster, nil
}

func DescribeCluster(ctx context.Context, s *Session, name string) (*AwsCluster, error) {
	return compute.GetValue[*AwsCluster](ctx, &cachedDescribeCluster{session: s, name: name})
}

type AwsCluster struct {
	Cluster         *types.Cluster
	HasOidcProvider bool
}

type cachedDescribeCluster struct {
	session *Session
	name    string

	compute.DoScoped[*AwsCluster]
}

func (cd *cachedDescribeCluster) Action() *tasks.ActionEvent {
	return tasks.Action("eks.describe-cluster").Category("aws").Arg("name", cd.name)
}

func (cd *cachedDescribeCluster) Inputs() *compute.In {
	return compute.Inputs().Str("session", cd.session.sesh.CacheKey()).Str("name", cd.name)
}

func (cd *cachedDescribeCluster) Output() compute.Output { return compute.Output{NotCacheable: true} }

func (cd *cachedDescribeCluster) Compute(ctx context.Context, _ compute.Resolved) (*AwsCluster, error) {
	// Doing two requests to AWS in parallel since each takes 600-700ms.
	// Total time is still around 1000-1100ms.
	eg := executor.New(ctx, "eks.describe-cluster")

	var output eks.DescribeClusterOutput
	eg.Go(func(ctx context.Context) error {
		out, err := cd.session.eks.DescribeCluster(ctx, &eks.DescribeClusterInput{
			Name: &cd.name,
		})
		if err != nil {
			return auth.CheckNeedsLoginOr(cd.session.sesh, err, func(err error) error {
				return fnerrors.New("eks: describe cluster failed: %w", err)
			})
		}

		if out.Cluster == nil {
			return fnerrors.InvocationError("api didn't return a cluster description as expected")
		}

		output = *out
		return nil
	})

	var oidcProviders iam.ListOpenIDConnectProvidersOutput
	eg.Go(func(ctx context.Context) error {
		providers, err := cd.session.iam.ListOpenIDConnectProviders(ctx, &iam.ListOpenIDConnectProvidersInput{})
		if err != nil {
			return auth.CheckNeedsLoginOr(cd.session.sesh, err, func(err error) error {
				return fnerrors.InvocationError("failed to list OpenID Connect providers: %w", err)
			})
		}

		oidcProviders = *providers

		return nil
	})

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	hasOidcProvider := false
	issuerParts := strings.Split(*output.Cluster.Identity.Oidc.Issuer, "/")
	issuerId := issuerParts[len(issuerParts)-1]
	for _, oidcProvider := range oidcProviders.OpenIDConnectProviderList {
		if strings.HasSuffix(*oidcProvider.Arn, issuerId) {
			hasOidcProvider = true
			break
		}
	}

	return &AwsCluster{
		Cluster:         output.Cluster,
		HasOidcProvider: hasOidcProvider,
	}, nil
}
