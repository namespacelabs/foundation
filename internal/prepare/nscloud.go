// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

func NamespaceCluster(machineType string, ephemeral, withBuild bool) Stage {
	return Stage{
		Pre: func(ch chan *orchestration.Event) {
			ch <- &orchestration.Event{
				Category:      "Namespace Cloud",
				ResourceId:    "new-cluster",
				ResourceLabel: "New cluster",
			}

			if withBuild {
				ch <- &orchestration.Event{
					Category:      "Namespace Cloud",
					ResourceId:    "build-cluster",
					ResourceLabel: "Setup build cluster",
				}
			}
		},

		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			return PrepareNewNamespaceCluster(ctx, env, machineType, ephemeral, withBuild, ch)
		},
	}
}

func PrepareNewNamespaceCluster(ctx context.Context, env cfg.Context, machineType string, ephemeral, withBuild bool, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
	eg := executor.New(ctx, "prepare-new-cluster")

	var mainMessages, buildMessages []proto.Message

	eg.Go(func(ctx context.Context) error {
		cfg, err := api.CreateAndWaitCluster(ctx, api.Endpoint, api.CreateClusterOpts{
			MachineType: machineType,
			Ephemeral:   ephemeral,
			KeepAlive:   true,
			Purpose:     env.Environment().Name})
		if err != nil {
			return err
		}

		mainMessages = append(mainMessages, &configuration.Cluster{ClusterId: cfg.ClusterId})
		return nil
	})

	if withBuild {
		eg.Go(func(ctx context.Context) error {
			msg, err := nscloud.EnsureBuildCluster(ctx, api.Endpoint)
			if err != nil {
				return err
			}

			ch <- &orchestration.Event{
				Category:      "Namespace Cloud",
				ResourceId:    "build-cluster",
				ResourceLabel: "Setup build cluster",
				Ready:         orchestration.Event_READY,
				Stage:         orchestration.Event_DONE,
			}

			buildMessages = append(buildMessages, msg)
			return nil
		})
	}

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
