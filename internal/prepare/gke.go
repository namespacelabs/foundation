// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/gcp/gke"
)

func PrepareGkeCluster(clusterName string) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:      "Google Cloud",
				ResourceId:    "gke-cluster",
				Scope:         fmt.Sprintf("Configure GKE Cluster %q", clusterName), // XXX remove soon.
				ResourceLabel: fmt.Sprintf("Configure GKE Cluster %q", clusterName),
			}
		},
		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			ch <- &orchestration.Event{
				ResourceId: "gke-cluster",
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return devhost.MakeConfiguration(
				&client.HostEnv{Provider: "gcp/gke"},
				&registry.Provider{Provider: "gcp/artifactregistry"},
				&gke.Cluster{Name: clusterName},
			)
		},
	}
}
