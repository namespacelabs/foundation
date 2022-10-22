// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ecr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/internal/providers/aws/auth"
	"namespacelabs.dev/foundation/std/tasks"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

type defaultKeychain struct{}

func (dk defaultKeychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	// XXX rethink this; we need more context in order to pick the right credentials.
	session, err := awsprovider.ConfiguredSession(ctx, nil)
	if err != nil {
		return nil, err
	}

	config, err := keychainSession{sesh: session}.refreshPrivateAuth(ctx)
	if err != nil {
		return nil, err
	}

	if config.ServerAddress == r.RegistryStr() {
		return authn.FromConfig(authn.AuthConfig{
			Username: config.Username,
			Password: config.Password,
		}), nil
	}

	// Nothing available.
	return authn.FromConfig(authn.AuthConfig{}), nil
}

type keychainSession struct {
	sesh *awsprovider.Session
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
	if em.sesh == nil {
		return nil, fnerrors.New("aws/ecr: no credentials available")
	}

	return tasks.Return(ctx, tasks.Action("aws.ecr.auth"),
		func(ctx context.Context) (*dockertypes.AuthConfig, error) {
			return refreshAuth(ctx,
				func(ctx context.Context) ([]types.AuthorizationData, error) {
					resp, err := compute.GetValue[*ecr.GetAuthorizationTokenOutput](ctx, &cachedAuthToken{sesh: em.sesh})
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

					return repoURL(em.sesh.Config(), res.Value), nil
				})
		})
}

func (em keychainSession) resolveAccount() compute.Computable[*sts.GetCallerIdentityOutput] {
	return auth.ResolveWithConfig(em.sesh)
}

type cachedAuthToken struct {
	sesh *awsprovider.Session

	compute.DoScoped[*ecr.GetAuthorizationTokenOutput]
}

func (cat cachedAuthToken) Action() *tasks.ActionEvent {
	return tasks.Action("ecr.get-auth-token").Category("aws")
}

func (cat cachedAuthToken) Inputs() *compute.In {
	return compute.Inputs().Str("cacheKey", cat.sesh.CacheKey())
}

func (cat cachedAuthToken) Compute(ctx context.Context, _ compute.Resolved) (*ecr.GetAuthorizationTokenOutput, error) {
	token, err := ecr.NewFromConfig(cat.sesh.Config()).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, auth.CheckNeedsLoginOr(cat.sesh, err, func(err error) error {
			return fnerrors.New("ecr: get auth token failed: %w", err)
		})
	}

	return token, nil
}
