// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ecr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
)

func ProvideClient(ctx context.Context, _ *ClientArgs, deps ExtensionDeps) (*eks.Client, error) {
	cfg, err := deps.ClientFactory.New(ctx)
	if err != nil {
		return nil, err
	}

	return eks.NewFromConfig(cfg), nil
}
