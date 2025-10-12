// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package client

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration/server/constants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	orchestratorStateKey = "foundation.orchestration"
	ConnTimeout          = time.Minute // TODO reduce - we've seen slow connections in CI
)

// UseOrchestrator controls whether to deploy the orchestrator.
// Historically this was used for service readiness checks, but that functionality has been removed.
// The orchestrator still provides Kubernetes controllers for runtime config management.
var UseOrchestrator = false

type remoteOrchestrator struct {
	cluster runtime.ClusterNamespace
	server  runtime.Deployable
}

func RemoteOrchestrator(cluster runtime.ClusterNamespace, server runtime.Deployable) *remoteOrchestrator {
	return &remoteOrchestrator{cluster: cluster, server: server}
}

func (c *remoteOrchestrator) Connect(ctx context.Context) (*grpc.ClientConn, error) {
	orch := compute.On(ctx)
	sink := tasks.SinkFrom(ctx)

	return grpc.NewClient("passthrough:///orchestrator",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			patchedContext := compute.AttachOrch(tasks.WithSink(ctx, sink), orch)

			conn, err := c.cluster.DialServer(patchedContext, c.server, &schema.Endpoint_Port{Name: constants.PortName})
			if err != nil {
				fmt.Fprintf(console.Debug(patchedContext), "failed to dial orchestrator: %v\n", err)
				return nil, err
			}

			return conn, nil
		}),
	)
}

func RegisterOrchestrator(prepare func(ctx context.Context, target cfg.Configuration, cluster runtime.Cluster) (any, error)) {
	if !UseOrchestrator {
		return
	}

	runtime.RegisterPrepare(orchestratorStateKey, prepare)
}

func ConnectToOrchestrator(ctx context.Context, cluster runtime.Cluster) (*grpc.ClientConn, error) {
	raw, err := cluster.EnsureState(ctx, orchestratorStateKey)
	if err != nil {
		return nil, err
	}

	return raw.(*remoteOrchestrator).Connect(ctx)
}
