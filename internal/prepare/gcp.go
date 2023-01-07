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
	"namespacelabs.dev/foundation/universe/gcp"
)

func PrepareGcpProjectID(projectID string) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:      "Google Cloud",
				ResourceId:    "gcp-projectid",
				Scope:         fmt.Sprintf("Configure Project ID %q", projectID), // XXX remove soon.
				ResourceLabel: fmt.Sprintf("Configure Project ID %q", projectID),
			}
		},

		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			hostEnv := &gcp.Project{
				Id: projectID,
			}
			c, err := devhost.MakeConfiguration(hostEnv)
			if err != nil {
				return nil, err
			}

			ch <- &orchestration.Event{
				ResourceId: "gcp-projectid",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return c, nil
		},
	}
}
