// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package auth

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func ResolveWithConfig(session *awsprovider.Session) compute.Computable[*sts.GetCallerIdentityOutput] {
	return &resolveAccount{Session: session}
}

type resolveAccount struct {
	Session *awsprovider.Session

	compute.DoScoped[*sts.GetCallerIdentityOutput]
}

func (r *resolveAccount) Action() *tasks.ActionEvent {
	return tasks.Action("sts.get-caller-identity").Category("aws")
}

func (r *resolveAccount) Inputs() *compute.In {
	return compute.Inputs().Str("cacheKey", r.Session.CacheKey())
}

func (r *resolveAccount) Compute(ctx context.Context, _ compute.Resolved) (*sts.GetCallerIdentityOutput, error) {
	out, err := sts.NewFromConfig(r.Session.Config()).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		var e *ssocreds.InvalidTokenError
		if errors.As(err, &e) {
			if usage := r.Session.RefreshUsage(); usage != "" {
				return nil, fnerrors.UsageError(usage, "AWS session credentials have expired.")
			}

			return nil, fnerrors.New("AWS session credentials are invalid")
		}

		return nil, err
	}

	if out.Account == nil {
		return nil, fnerrors.InvocationError("expected GetCallerIdentityOutput.Account to be present")
	}

	return out, nil
}
