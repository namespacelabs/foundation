// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ecr

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

func ProvideClient(ctx context.Context, _ *ClientArgs, deps ExtensionDeps) (*ecr.Client, error) {
	cfg, err := deps.ClientFactory.New(ctx)
	if err != nil {
		return nil, err
	}

	return ecr.NewFromConfig(cfg), nil
}
