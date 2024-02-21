// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/slack-go/slack"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/orchestration/client"
	"namespacelabs.dev/foundation/schema"
	orchpb "namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

// Bumping this value leads to an orchestrator upgrade.
const orchestratorVersion = 25

var DeployWithOrchestrator = false
var DeployUpdateSlackChannel, SlackToken string

func ExecuteOpts() execution.ExecuteOpts {
	return execution.ExecuteOpts{
		ContinueOnErrors:    false,
		OrchestratorVersion: orchestratorVersion,
	}
}

func getSlackTokenAndChannel(ctx context.Context, cfg cfg.Context, env *schema.Environment) (string, string, error) {
	if DeployUpdateSlackChannel != "" {
		return os.ExpandEnv(SlackToken), os.ExpandEnv(DeployUpdateSlackChannel), nil
	}

	if channel := env.Policy.GetDeployUpdateSlackChannel(); channel != "" {
		// TODO
		return "", channel, nil
	}

	return "", "", nil
}

func Deploy(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, plan *schema.DeployPlan, reason string, wait, outputProgress bool) error {
	if !DeployWithOrchestrator {
		if !wait {
			return fnerrors.BadInputError("waiting is mandatory without the orchestrator")
		}

		observeError := func(context.Context, error) {}
		if token, channel, err := getSlackTokenAndChannel(ctx, env, plan.Environment); err != nil {
		} else if channel != "" {
			if token == "" {
				return fnerrors.BadInputError("a slack token is required to be able to update a channel")
			}

			start := time.Now()
			slackcli := slack.New(token)
			chid, ts, err := slackcli.PostMessageContext(ctx, channel, slack.MsgOptionBlocks(renderSlackMessage(plan, start, time.Time{}, reason, nil)...))
			if err != nil {
				fmt.Fprintf(console.Warnings(ctx), "Failed to post to Slack: %v\n", err)
			} else {
				observeError = func(ctx context.Context, err error) {
					if _, _, _, err := slackcli.UpdateMessageContext(ctx, chid, ts, slack.MsgOptionBlocks(renderSlackMessage(plan, start, time.Now(), reason, err)...)); err != nil {
						fmt.Fprintf(console.Warnings(ctx), "Failed to update Slack: %v\n", err)
					}
				}
			}
		}

		p := execution.NewPlan(plan.Program.Invocation...)

		// Make sure that the cluster is accessible to a serialized invocation implementation.
		execErr := execution.ExecuteExt(ctx, "deployment.execute", p,
			deploy.MaybeRenderBlock(env, cluster, outputProgress),
			ExecuteOpts(),
			execution.FromContext(env),
			runtime.InjectCluster(cluster))
		observeError(ctx, execErr)
		return execErr
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

			conn, err := client.ConnectToOrchestrator(ctx, cluster.Cluster())
			if err != nil {
				return err
			}

			defer conn.Close()

			id, err := client.CallDeploy(ctx, env, conn, plan)
			if err != nil {
				return err
			}

			if wait {
				var ch chan *orchpb.Event
				var cleanup func(ctx context.Context) error

				if outputProgress {
					ch, cleanup = deploy.MaybeRenderBlock(env, cluster, true)(ctx)
				}

				err := client.WireDeploymentStatus(ctx, conn, id, ch)
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
