// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
)

func Orchestrator() ClusterStage {
	return ClusterStage{
		Pre: func(ch chan *orchpb.Event) {
			ch <- &orchpb.Event{
				ResourceId: "orchestrator",
				Scope:      "Deploy Namespace Orchestrator",
				Category:   "Preparing cluster",
				Ready:      orchpb.Event_NOT_READY,
				Stage:      orchpb.Event_WAITING,
			}
		},
		Post: func(ch chan *orchpb.Event) {
			ch <- &orchpb.Event{
				ResourceId: "orchestrator",
				Ready:      orchpb.Event_READY,
				Stage:      orchpb.Event_DONE,
			}
		},
		Run: func(ctx context.Context, env cfg.Context, devhost *schema.DevHost_ConfigureEnvironment, kube *kubernetes.Cluster, ch chan *orchpb.Event) error {
			return PrepareOrchestratorInKube(ctx, env, devhost, kube)
		},
	}
}

func PrepareOrchestratorInKube(ctx context.Context, env cfg.Context, devhost *schema.DevHost_ConfigureEnvironment, kube *kubernetes.Cluster) error {
	config, err := cfg.MakeConfigurationCompat(env, env.Workspace(), &schema.DevHost{
		Configure: []*schema.DevHost_ConfigureEnvironment{devhost}}, env.Environment())
	if err != nil {
		return err
	}

	if _, err := orchestration.PrepareOrchestrator(ctx, config, kube, false); err != nil {
		return err
	}

	return nil
}
