// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	awsconf "namespacelabs.dev/foundation/universe/aws/configuration"
)

func PrepareAWSProfile(profileName string) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:   "AWS",
				ResourceId: "aws-profile",
				Scope:      fmt.Sprintf("Configure AWS profile %q", profileName),
			}
		},

		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &awsconf.Configuration{
				Profile: profileName,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}

			ch <- &orchestration.Event{
				ResourceId: "aws-profile",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return c, nil
		},
	}
}
