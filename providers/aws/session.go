// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func ConfiguredSession(ctx context.Context, env ops.Environment) (aws.Config, string, error) {
	conf := &Conf{}
	if !devhost.ConfigurationForEnv(env).Get(conf) {
		return aws.Config{}, "", fnerrors.UsageError(fmt.Sprintf("Run `fn prepare --env=%s`.", env.Proto().GetName()), "Foundation has not been configured to access AWS.")
	}

	profile := conf.Profile
	if profile == "" {
		profile = "default"
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	return cfg, profile, err
}