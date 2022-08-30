// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	serverPkg  = "namespacelabs.dev/foundation/internal/orchestration/server"
	servicePkg = "namespacelabs.dev/foundation/internal/orchestration/service"
)

type clientInstance struct {
	env provision.Env

	compute.DoScoped[proto.OrchestrationServiceClient] // Only connect once per configuration.
}

func ConnectToClient(env provision.Env) compute.Computable[proto.OrchestrationServiceClient] {
	return &clientInstance{env: env}
}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("orchestrator.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs()
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (proto.OrchestrationServiceClient, error) {
	focus, err := c.env.RequireServer(ctx, schema.PackageName(serverPkg))
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployServers(ctx, c.env, []provision.Server{focus}, nil)
	if err != nil {
		return nil, err
	}

	computed, err := compute.GetValue(ctx, plan)
	if err != nil {
		return nil, err
	}

	waiters, err := computed.Deployer.Execute(ctx, runtime.TaskServerDeploy, c.env)
	if err != nil {
		return nil, err
	}

	// Don't render a wait block here.
	if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
		return nil, err
	}

	endpoint := &schema.Endpoint{}
	for _, e := range computed.ComputedStack.Endpoints {
		if e.EndpointOwner != servicePkg {
			continue
		}
		for _, meta := range e.ServiceMetadata {
			if meta.Kind == proto.OrchestrationService_ServiceDesc.ServiceName {
				endpoint = e
			}
		}
	}

	rt := runtime.For(ctx, c.env)

	portch := make(chan runtime.ForwardedPort)

	defer close(portch)
	if _, err := rt.ForwardPort(ctx, focus.Proto(), endpoint.Port.ContainerPort, []string{"127.0.0.1"}, func(fp runtime.ForwardedPort) {
		portch <- fp
	}); err != nil {
		return nil, err
	}

	port, ok := <-portch
	if !ok {
		return nil, fnerrors.InternalError("unexpected error")
	}

	conn, err := grpc.DialContext(ctx, fmt.Sprintf("127.0.0.1:%d", port.LocalPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fnerrors.Wrap(focus.Location, err)
	}

	cli := proto.NewOrchestrationServiceClient(conn)

	return cli, nil
}

func Deploy(ctx context.Context, env provision.Env, plan *schema.DeployPlan) (string, error) {
	req := &proto.DeployRequest{
		Plan: plan,
	}

	cli, err := compute.GetValue(ctx, ConnectToClient(env))
	if err != nil {
		return "", err
	}

	resp, err := cli.Deploy(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Id, nil
}
