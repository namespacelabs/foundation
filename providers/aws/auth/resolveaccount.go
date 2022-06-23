// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ResolveWithProfile(ctx context.Context, profile string) (compute.Computable[*sts.GetCallerIdentityOutput], error) {
	if profile == "" {
		profile = "default"
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, err
	}

	return ResolveWithConfig(cfg, profile), nil
}

func ResolveWithConfig(config aws.Config, profile string) compute.Computable[*sts.GetCallerIdentityOutput] {
	return &resolveAccount{Config: config, Profile: profile}
}

type resolveAccount struct {
	Config  aws.Config // Doesn't affect output.
	Profile string     // Used purely as cache key.

	compute.DoScoped[*sts.GetCallerIdentityOutput]
}

func (r *resolveAccount) Action() *tasks.ActionEvent {
	return tasks.Action("sts.get-caller-identity").Category("aws")
}

func (r *resolveAccount) Inputs() *compute.In { return compute.Inputs().Str("profile", r.Profile) }

func (r *resolveAccount) Compute(ctx context.Context, _ compute.Resolved) (*sts.GetCallerIdentityOutput, error) {
	out, err := sts.NewFromConfig(r.Config).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		var e *ssocreds.InvalidTokenError
		if errors.As(err, &e) {
			return nil, fnerrors.UsageError(
				fmt.Sprintf("Try running `aws --profile %s sso login`.", r.Profile),
				"AWS session credentials have expired.")
		}

		return nil, err
	}

	if out.Account == nil {
		return nil, fnerrors.InvocationError("expected GetCallerIdentityOutput.Account to be present")
	}

	return out, nil
}
