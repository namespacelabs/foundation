// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

func NamespaceCluster(machineType string, ephemeral bool) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:      "Namespace Cloud",
				ResourceId:    "new-cluster",
				ResourceLabel: "New cluster",
			}

		},

		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			return PrepareNewNamespaceCluster(ctx, env, machineType, ephemeral, ch)
		},
	}
}

func PrepareNewNamespaceCluster(ctx context.Context, env cfg.Context, machineType string, ephemeral bool, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
	eg := executor.New(ctx, "prepare-new-cluster")

	var mainMessages, buildMessages []proto.Message

	eg.Go(func(ctx context.Context) error {
		cfg, err := api.CreateAndWaitCluster(ctx, api.Methods, time.Minute, api.CreateClusterOpts{
			MachineType:     machineType,
			KeepAtExit:      true,
			Purpose:         env.Environment().Name,
			WaitClusterOpts: api.WaitClusterOpts{WaitKind: "kubernetes"},
			Experimental: map[string]any{
				"k3s": private.K3sCfg,
			},
		})
		if err != nil {
			return err
		}

		mainMessages = append(mainMessages, &configuration.Cluster{ClusterId: cfg.ClusterId, ApiEndpoint: cfg.Cluster.ApiEndpoint})
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	ch <- &orchestration.Event{
		Category:      "Namespace Cloud",
		ResourceId:    "new-cluster",
		ResourceLabel: "New cluster",
		Ready:         orchestration.Event_READY,
		Stage:         orchestration.Event_DONE,
	}

	return devhost.MakeConfiguration(append(mainMessages, buildMessages...)...)
}
