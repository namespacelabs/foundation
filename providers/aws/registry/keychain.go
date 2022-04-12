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
	"namespacelabs.dev/foundation/internal/fnerrors"
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

func (em keychainSession) refreshPrivateAuth(ctx context.Context) (authcfg *dockertypes.AuthConfig, err error) {
	err = tasks.Action("aws.ecr.auth").Run(ctx, func(ctx context.Context) error {
		var err error
		authcfg, err = refreshAuth(ctx,
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
		return err
	})
	return
}

func (em keychainSession) resolveAccount() compute.Computable[*sts.GetCallerIdentityOutput] {
	return &resolveAccount{sesh: em.sesh, profile: em.profile}
}

type resolveAccount struct {
	sesh    aws.Config // Doesn't affect output.
	profile string

	compute.DoScoped[*sts.GetCallerIdentityOutput]
}

func (r *resolveAccount) Action() *tasks.ActionEvent {
	return tasks.Action("sts.get-caller-identity").Category("aws")
}
func (r *resolveAccount) Inputs() *compute.In { return compute.Inputs().Str("profile", r.profile) }
func (r *resolveAccount) Compute(ctx context.Context, _ compute.Resolved) (*sts.GetCallerIdentityOutput, error) {
	out, err := sts.NewFromConfig(r.sesh).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, err
	}

	if out.Account == nil {
		return nil, fnerrors.InvocationError("expected GetCallerIdentityOutput.Account to be present")
	}

	return out, nil
}
