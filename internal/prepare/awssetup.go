// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareAWSRegistry(registryName string, env ops.Environment) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.aws-registry").HumanReadablef("Prepare the AWS registry configuration"),
		compute.Inputs().Str("registryName", registryName).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true, NonDeterministic: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			p := &registry.Provider{
				Provider: registryName,
			}
			c, err := devhost.MakeConfiguration(p)
			if err != nil {
				return nil, err
			}
			c.Purpose = env.Proto().GetPurpose()
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}

func PrepareAWSProfile(profileName string, env ops.Environment) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.aws-profile").HumanReadablef("Prepare the AWS profile configuration"),
		compute.Inputs().Str("profileName", profileName).Proto("env", env.Proto()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &aws.Conf{
				Profile: profileName,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}
			c.Purpose = env.Proto().GetPurpose()
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
