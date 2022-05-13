// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/providers/aws/auth"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

type defaultKeychain struct{}

func (dk defaultKeychain) Resolve(ctx context.Context, authn authn.Resource) (authn.Authenticator, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	// XXX need devhost to get a profile.
	return keychainSession{cfg, "default"}.Resolve(ctx, authn)
}

type keychainSession struct {
	sesh    aws.Config
	profile string
}

var _ oci.Keychain = keychainSession{}

func (em keychainSession) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	config, err := em.refreshPrivateAuth(ctx)
	if err != nil {
		return nil, err
	}

	if config.ServerAddress == r.RegistryStr() {
		return authn.FromConfig(authn.AuthConfig{
			Username: config.Username,
			Password: config.Password,
		}), nil
	}

	return nil, nil
}

func (em keychainSession) refreshPrivateAuth(ctx context.Context) (*dockertypes.AuthConfig, error) {
	return tasks.Return(ctx, tasks.Action("aws.ecr.auth").Arg("profile", em.profile),
		func(ctx context.Context) (*dockertypes.AuthConfig, error) {
			return refreshAuth(ctx,
				func(ctx context.Context) ([]types.AuthorizationData, error) {
					resp, err := ecr.NewFromConfig(em.sesh).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
					if err != nil {
						return nil, err
					}
					return resp.AuthorizationData, nil
				},
				func(ctx context.Context) (string, error) {
					res, err := compute.Get(ctx, em.resolveAccount())
					if err != nil {
						return "", err
					}

					return repoURL(em.sesh, res.Value), nil
				})
		})
}

func (em keychainSession) resolveAccount() compute.Computable[*sts.GetCallerIdentityOutput] {
	return auth.ResolveWith(em.sesh, em.profile)
}
