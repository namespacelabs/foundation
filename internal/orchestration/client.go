// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/orchestration/service/proto"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type clientInstance struct {
	env ops.Environment

	compute.DoScoped[*proto.OrchestrationServiceClient] // Only connect once per configuration.
}

func ConnectToClient(env ops.Environment) compute.Computable[proto.OrchestrationServiceClient] {
	return &clientInstance{env: env}
}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("orchestrator.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs()
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (proto.OrchestrationServiceClient, error) {
	var servers []provision.Server // TODO

	stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	plan, err := deploy.PrepareDeployStack(ctx, c.env, stack, servers)
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

	// TODO: port forwarding!
	endpointAddress := "TODO"
	conn, err := grpc.DialContext(ctx, endpointAddress)
	if err != nil {
		return nil, err
	}

	cli := proto.NewOrchestrationServiceClient(conn)

	return cli, nil
}

func Deploy(ctx context.Context, env ops.Environment, plan *schema.DeployPlan) (string, error) {
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
