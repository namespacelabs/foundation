// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	awsconf "namespacelabs.dev/foundation/universe/aws/configuration"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func PrepareAWSProfile(env planning.Context, profileName string) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.aws-profile").HumanReadablef("Prepare the AWS profile configuration"),
		compute.Inputs().Str("profileName", profileName).Proto("env", env.Environment()),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &awsconf.Configuration{
				Profile: profileName,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}
			c.Name = env.Environment().GetName()
			return []*schema.DevHost_ConfigureEnvironment{c}, nil
		})
}
