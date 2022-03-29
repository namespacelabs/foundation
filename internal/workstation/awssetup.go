// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workstation

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

func PrepareAWSRegistry(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
	p := &registry.Provider{
		Provider: "aws/ecr",
	}

	c, err := devhost.MakeConfiguration(p)
	if err != nil {
		return nil, err
	}
	c.Purpose = env.Proto().GetPurpose()

	return []*schema.DevHost_ConfigureEnvironment{c}, nil
}

func SetAWSProfile(name string) PrepareFunc {
	return func(ctx context.Context, env ops.Environment, pr *PrepareResult) ([]*schema.DevHost_ConfigureEnvironment, error) {
		hostEnv := &aws.Conf{
			Profile: name,
		}

		c, err := devhost.MakeConfiguration(hostEnv)
		if err != nil {
			return nil, err
		}
		c.Purpose = env.Proto().GetPurpose()
		return []*schema.DevHost_ConfigureEnvironment{c}, nil
	}
}