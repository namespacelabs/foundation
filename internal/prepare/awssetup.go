// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
	awsconf "namespacelabs.dev/foundation/universe/aws/configuration"
)

func PrepareAWSProfile(profileName string) compute.Computable[*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.aws-profile").HumanReadablef("Attaching AWS profile to Namespace's configuration"),
		compute.Inputs().Str("profileName", profileName),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) (*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &awsconf.Configuration{
				Profile: profileName,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}

			fmt.Fprintf(console.Stdout(ctx), "[✓] Configure Namespace to use AWS profile %q.\n", profileName)

			return c, nil
		})
}
