// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"context"
	"fmt"
	"os"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/planning"
)

func RunAttached(ctx context.Context, config planning.Context, cluster ClusterNamespace, spec DeployableSpec, io TerminalIO) error {
	plan, err := cluster.Planner().PlanDeployment(ctx, DeploymentSpec{
		Specs: []DeployableSpec{spec},
	})
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
	if err := execution.Execute(ctx, config, "deployable.run-attached", g, nil, InjectCluster(cluster)...); err != nil {
		return fnerrors.New("failed to deploy: %w", err)
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

func RunAttachedStdio(ctx context.Context, config planning.Context, cluster ClusterNamespace, spec DeployableSpec) error {
	return RunAttached(ctx, config, cluster, spec, TerminalIO{
		TTY:    true,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
}
