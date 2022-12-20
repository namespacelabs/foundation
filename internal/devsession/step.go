// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"fmt"
	"sync"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/config"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/planning/snapshot"
	"namespacelabs.dev/foundation/internal/portforward"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"

	"namespacelabs.dev/foundation/std/tasks"
)

func setWorkspace(ctx context.Context, env cfg.Context, rt runtime.ClusterNamespace, packageNames []string, session *Session, portForward *portforward.PortForward) error {
	planner, err := runtime.PlannerFor(ctx, env)
	if err != nil {
		return err
	}

	return compute.Do(ctx, func(ctx context.Context) error {
		serverPackages := schema.PackageNames(packageNames...)
		focusServers := snapshot.RequireServers(env, serverPackages...)

		fmt.Fprintf(console.Debug(ctx), "devworkflow: setWorkspace.Do\n")

		// Changing the graph is fairly heavy-weight at this point, as it will lead to
		// a rebuild of all packages (although they'll likely hit the cache), as well
		// as a full re-deployment, re-port forward, etc. Ideally this would be more
		// incremental by having narrower dependencies. E.g. single server would have
		// a single build, single deployment, etc. And changes to siblings servers
		// would only impact themselves, not all servers. #362
		if err := compute.Continuously(ctx, &buildAndDeploy{
			session:        session,
			portForward:    portForward,
			env:            env,
			serverPackages: serverPackages,
			focusServers:   focusServers,
			cluster:        rt,
			planner:        planner,
		}, nil); err != nil {
			return err
		}

		return nil
	})
}

type buildAndDeploy struct {
	session        *Session
	portForward    *portforward.PortForward
	env            cfg.Context
	serverPackages []schema.PackageName
	focusServers   compute.Computable[*snapshot.ServerSnapshot]
	cluster        runtime.ClusterNamespace
	planner        runtime.Planner

	mu            sync.Mutex
	cancelRunning func()
}

func (do *buildAndDeploy) Inputs() *compute.In {
	return compute.Inputs().Computable("focusServers", do.focusServers)
}

func (do *buildAndDeploy) Updated(ctx context.Context, r compute.Resolved) error {
	fmt.Fprintf(console.Debug(ctx), "devworkflow: buildAndDeploy.Updated\n")

	do.mu.Lock()
	defer do.mu.Unlock()

	// If a previous run is on-going, cancel it.
	if do.cancelRunning != nil {
		do.cancelRunning()
		do.cancelRunning = nil
	}

	snapshot := compute.MustGetDepValue(r, do.focusServers, "focusServers")
	focus, err := snapshot.Get(do.serverPackages...)
	if err != nil {
		return err
	}

	do.session.updateStackInPlace(func(stack *Stack) {
		resetStack(stack, do.env, do.session.availableEnvs, focus)
	})

	switch do.env.Environment().Purpose {
	case schema.Environment_DEVELOPMENT, schema.Environment_TESTING:
		var observers []integrations.DevObserver

		defer func() {
			for _, obs := range observers {
				obs.Close()
			}
		}()

		if do.env.Environment().Purpose == schema.Environment_DEVELOPMENT {
			for _, srv := range focus {
				var observer integrations.DevObserver

				// Must be invoked before building to make sure stack computation and building
				// uses the updated context.
				ctx, observer, err = integrations.IntegrationFor(srv.Framework()).PrepareDev(ctx, do.cluster, srv)
				if err != nil {
					return err
				}

				if observer != nil {
					observers = append(observers, observer)
				}
			}
		}

		stack, err := planning.ComputeStack(ctx, focus, planning.ProvisionOpts{PortRange: eval.DefaultPortRange()})
		if err != nil {
			return err
		}

		do.session.updateStackInPlace(func(s *Stack) {
			s.Stack = stack.Proto()
		})

		observers = append(observers, updateDeploymentStatus{do.session})

		p, err := planning.NewPlannerFromExisting(do.env, do.planner)
		if err != nil {
			return err
		}

		plan, err := deploy.PrepareDeployStack(ctx, p, stack)
		if err != nil {
			return err
		}

		// The actual build + deploy is deferred into a separate Continuously() call, which
		// reacts to changes to the dependencies of build/deploy (e.g. sources). We can't
		// block here either or else we won't have updates to the package graph to be
		// delivered (Continuously doesn't call Updated, until the previous call returns).

		// A channel is used to signal that the child Continuously() has returned, and
		// thus we can be sure that its Cleanup has been called.
		done := make(chan struct{})
		transformError := func(err error) error {
			if err != nil {
				if msg, ok := fnerrors.IsExpected(err); ok {
					fmt.Fprintf(console.Stderr(ctx), "\n  %s\n\n", msg)
					return nil // Swallow the error in case it is expected.
				}
			}
			return err
		}
		cancel := compute.SpawnCancelableOnContinuously(ctx, func(ctx context.Context) error {
			defer close(done)
			return compute.Continuously(ctx,
				newUpdateCluster(snapshot.Env(), do.cluster, stack.Proto(), do.serverPackages, observers, plan, do.portForward),
				transformError)
		})

		do.cancelRunning = func() {
			cancel()
			<-done
		}

	case schema.Environment_PRODUCTION:
		if len(focus) > 1 {
			fmt.Fprintf(console.Stderr(ctx), "Ignoring the following servers when fetching production configuration: %s\n", do.serverPackages[1:])
		}

		server := focus[0]
		if err := tasks.Action("stack.rehydrate").Scope(server.PackageName()).Run(ctx,
			func(ctx context.Context) error {
				buildID, err := do.cluster.DeployedConfigImageID(ctx, server.Proto())
				if err != nil {
					return err
				}

				s, err := config.Rehydrate(ctx, server, buildID)
				if err != nil {
					return err
				}

				do.session.updateStackInPlace(func(stack *Stack) {
					stack.Stack = s.Stack
				})

				return nil
			}); err != nil {
			return err
		}
	}

	return nil
}

type updateDeploymentStatus struct {
	session *Session
}

func (u updateDeploymentStatus) OnDeployment(ctx context.Context) {
	u.session.updateStackInPlace(func(s *Stack) {
		s.Deployed = true
		s.DeploymentRevision++
	})
}

func (updateDeploymentStatus) Close() error {
	return nil
}

func (do *buildAndDeploy) Cleanup(ctx context.Context) error {
	do.mu.Lock()
	defer do.mu.Unlock()

	if do.cancelRunning != nil {
		do.cancelRunning()
	}

	return nil
}

func resetStack(out *Stack, env cfg.Context, availableEnvs []*schema.Environment, focus []planning.Server) {
	workspace := protos.Clone(env.Workspace().Proto())

	out.AbsRoot = env.Workspace().LoadedFrom().AbsPath
	out.Env = env.Environment()
	out.Workspace = workspace
	out.AvailableEnv = availableEnvs
	out.Deployed = false
	// The DeploymentRevision is purposely not reset. The contract is that it increases monotonically.

	out.Focus = nil
	out.Current = nil

	if len(focus) > 0 {
		out.Current = focus[0].StackEntry() // XXX legacy

		for _, fs := range focus {
			out.Focus = append(out.Focus, fs.PackageName().String())
		}
	}
}
