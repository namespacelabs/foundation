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
	"namespacelabs.dev/foundation/universe/aws/configuration/eks"
)

func PrepareEksCluster(clusterName string) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:   "AWS",
				ResourceId: "eks-profile",
				Scope:      fmt.Sprintf("Configure EKS Cluster %q", clusterName),
			}
		},
		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			ch <- &orchestration.Event{
				ResourceId: "eks-profile",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return devhost.MakeConfiguration(&eks.Cluster{Name: clusterName})
		},
	}
}
