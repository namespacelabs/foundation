// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"context"
	"fmt"
	"os"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
)

func RunAttached(ctx context.Context, config cfg.Context, planner Planner, spec DeployableSpec, io TerminalIO) error {
	plan, err := planner.PlanDeployment(ctx, DeploymentSpec{
		Specs: []DeployableSpec{spec},
	})
	if err != nil {
		return err
	}

	cluster, err := planner.EnsureClusterNamespace(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := cluster.DeleteDeployable(ctx, spec); err != nil {
			fmt.Fprintf(console.Errors(ctx), "Deleting %s failed: %v\n", spec.Name, err)
		}
	}()

	g := execution.NewPlan(plan.Definitions...)
	// ResolveContainers will wait until the deployable is running, so we don't rely on the waiters returned by Execute.
	if err := execution.Execute(ctx, "deployable.run-attached", g, nil, execution.FromContext(config), InjectCluster(cluster)); err != nil {
		return fnerrors.Newf("failed to deploy: %w", err)
	}

	containers, err := cluster.ResolveContainers(ctx, spec)
	if err != nil {
		return err
	}

	var mainContainers []*runtime.ContainerReference
	for _, container := range containers {
		if container.Kind == runtime.ContainerKind_PRIMARY {
			mainContainers = append(mainContainers, container)
		}
	}

	if len(mainContainers) != 1 {
		return fnerrors.InternalError("expected a single container, saw %d", len(mainContainers))
	}

	return cluster.Cluster().AttachTerminal(ctx, mainContainers[0], io)
}

func RunAttachedStdio(ctx context.Context, config cfg.Context, planner Planner, spec DeployableSpec) error {
	return RunAttached(ctx, config, planner, spec, TerminalIO{
		TTY:    true,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}
