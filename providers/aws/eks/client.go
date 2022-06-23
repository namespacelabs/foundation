// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

type Session struct {
	devHost  *schema.DevHost
	selector devhost.Selector
	sesh     aws.Config
	eks      *eks.Client
}

func NewSession(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	sesh, _, err := awsprovider.ConfiguredSession(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	return &Session{
		devHost:  devHost,
		selector: selector,
		sesh:     sesh,
		eks:      eks.NewFromConfig(sesh),
	}, nil
}
