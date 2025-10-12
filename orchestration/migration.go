// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package orchestration

import (
	"context"
	"fmt"
	"time"

	"github.com/slack-go/slack"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
)

var DeployUpdateSlackChannel, SlackToken string

func ExecuteOpts() execution.ExecuteOpts {
	return execution.ExecuteOpts{
		ContinueOnErrors: false,
	}
}

func Deploy(ctx context.Context, env cfg.Context, cluster runtime.ClusterNamespace, plan *schema.DeployPlan, reason string, wait, outputProgress bool) error {
	if !wait {
		return fnerrors.BadInputError("waiting is mandatory")
	}

	observeError := func(context.Context, error) {}
	if token, channel, err := resolveSlackTokenAndChannel(ctx, env); err != nil {
		return err
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
