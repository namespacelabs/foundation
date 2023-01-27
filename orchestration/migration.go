// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

// Bumping this value leads to an orchestrator upgrade.
const orchestratorVersion = 18

func ExecuteOpts() execution.ExecuteOpts {
	return execution.ExecuteOpts{
		ContinueOnErrors:    false,
		OrchestratorVersion: orchestratorVersion,
	}
}

func Deploy(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, plan *schema.DeployPlan, wait, outputProgress bool) error {
	if !UseOrchestrator {
		if !wait {
			return fnerrors.BadInputError("waiting is mandatory without the orchestrator")
		}

		p := execution.NewPlan(plan.Program.Invocation...)

		// Make sure that the cluster is accessible to a serialized invocation implementation.
		return execution.ExecuteExt(ctx, "deployment.execute", p,
			deploy.MaybeRenderBlock(env, cluster, outputProgress),
			ExecuteOpts(),
			execution.FromContext(env),
			runtime.InjectCluster(cluster))
	}

	return tasks.Action("orchestrator.deploy").Scope(schema.PackageNames(plan.FocusServer...)...).
		Run(ctx, func(ctx context.Context) error {
			debug := console.Debug(ctx)
			fmt.Fprintf(debug, "deploying program:\n")
			for k, inv := range plan.GetProgram().GetInvocation() {
				fmt.Fprintf(debug, " #%d %q --> cats:%v after:%v\n", k, inv.Description,
					inv.GetOrder().GetSchedCategory(),
					inv.GetOrder().GetSchedAfterCategory())
			}

			raw, err := cluster.Cluster().EnsureState(ctx, orchestratorStateKey)
			if err != nil {
				return err
			}

			conn, err := raw.(*RemoteOrchestrator).Connect(ctx)
			if err != nil {
				return err
			}

			defer conn.Close()

			id, err := CallDeploy(ctx, env, conn, plan)
			if err != nil {
				return err
			}

			if wait {
				var ch chan *orchpb.Event
				var cleanup func(ctx context.Context) error

				if outputProgress {
					ch, cleanup = deploy.MaybeRenderBlock(env, cluster, true)(ctx)
				}

				err := WireDeploymentStatus(ctx, conn, id, ch)
				if cleanup != nil {
					cleanupErr := cleanup(ctx)
					if err == nil {
						return cleanupErr
					}
				}

				return err
			}

			return nil
		})
}
