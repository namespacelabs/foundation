// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

type Session struct {
	env      *schema.Environment
	devHost  *schema.DevHost
	selector devhost.Selector
	sesh     *awsprovider.Session
	eks      *eks.Client
	iam      *iam.Client
}

func NewSession(ctx context.Context, env *schema.Environment, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	sesh, err := awsprovider.MustConfiguredSession(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	return &Session{
		env:      env,
		devHost:  devHost,
		selector: selector,
		sesh:     sesh,
		eks:      eks.NewFromConfig(sesh.Config()),
		iam:      iam.NewFromConfig(sesh.Config()),
	}, nil
}

func NewOptionalSession(ctx context.Context, env *schema.Environment, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	sesh, err := awsprovider.ConfiguredSession(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	if sesh == nil {
		return nil, err
	}

	return &Session{
		env:      env,
		devHost:  devHost,
		selector: selector,
		sesh:     sesh,
		eks:      eks.NewFromConfig(sesh.Config()),
		iam:      iam.NewFromConfig(sesh.Config()),
	}, nil
}
