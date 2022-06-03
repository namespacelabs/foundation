// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

func ComputeToken(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector, name string) (token.Token, token.Generator, error) {
	sess, _, err := aws.ConfiguredSessionV1(ctx, devHost, selector)
	if err != nil {
		return token.Token{}, nil, err
	}

	gen, err := token.NewGenerator(false, false)
	if err != nil {
		return token.Token{}, nil, fnerrors.New("could not get token: %w", err)
	}

	tok, err := gen.GetWithOptions(&token.GetTokenOptions{
		ClusterID:            name,
		AssumeRoleARN:        "", // Keeping these explicitly blank for future expansion.
		AssumeRoleExternalID: "",
		Session:              sess,
	})
	if err != nil {
		return token.Token{}, nil, fnerrors.New("could not get token: %w", err)
	}

	return tok, gen, nil
}
