// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Deploy(ctx context.Context, env planning.Context, cluster runtime.Cluster, p *ops.Plan, plan *schema.DeployPlan, wait, outputProgress bool) error {
	if !UseOrchestrator {
		if !wait {
			return fnerrors.BadInputError("waiting is mandatory without the orchestrator")
		}

		// Make sure that the cluster is accessible to a serialized invocation implementation.
		return ops.Execute(ctx, env.Configuration(), "deployment.execute", p,
			deploy.MaybeRenderBlock(env, cluster, outputProgress),
			runtime.ClusterInjection.With(cluster))
	}

	return tasks.Action("orchestrator.deploy").Run(ctx, func(ctx context.Context) error {
		raw, err := cluster.EnsureState(ctx, orchestratorStateKey)
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
			var handler = func(err error) error { return err }

			if outputProgress {
				ch, handler = deploy.MaybeRenderBlock(env, cluster, true)(ctx)
			}

			return handler(WireDeploymentStatus(ctx, conn, id, ch))
		}

		return nil
	})
}
