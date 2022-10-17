// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/std/cfg"
)

type Session struct {
	cfg  cfg.Configuration
	sesh *awsprovider.Session
	eks  *eks.Client
	iam  *iam.Client
}

func NewSession(ctx context.Context, cfg cfg.Configuration) (*Session, error) {
	sesh, err := awsprovider.MustConfiguredSession(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &Session{
		cfg:  cfg,
		sesh: sesh,
		eks:  eks.NewFromConfig(sesh.Config()),
		iam:  iam.NewFromConfig(sesh.Config()),
	}, nil
}

func NewOptionalSession(ctx context.Context, cfg cfg.Configuration) (*Session, error) {
	sesh, err := awsprovider.ConfiguredSession(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if sesh == nil {
		return nil, err
	}

	return &Session{
		cfg:  cfg,
		sesh: sesh,
		eks:  eks.NewFromConfig(sesh.Config()),
		iam:  iam.NewFromConfig(sesh.Config()),
	}, nil
}
