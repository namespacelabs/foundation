// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func doWorkspace(ctx context.Context, set *DevWorkflowRequest_SetWorkspace, obs *stackState) error {
	for {
		err := compute.Do(ctx, func(ctx context.Context) error {
			return step(ctx, set, obs)
		})

		if err != nil {
			if msg, ok := fnerrors.IsExpected(err); ok {
				fmt.Fprintf(console.Stderr(ctx), "\n  %s\n\n", msg)
				continue // Go again.
			}
		}

		return err
	}
}

func step(ctx context.Context, set *DevWorkflowRequest_SetWorkspace, obs *stackState) error {
	done := console.SetIdleLabel(ctx, "waiting for workspace changes")
	defer done()

	// Re-create loc/root here, to dump the cache.
	root, err := module.FindRoot(ctx, set.AbsRoot)
	if err != nil {
		return err
	}

	env, err := provision.RequireEnv(root, set.EnvName)
	if err != nil {
		return err
	}

	serverPackages := []schema.PackageName{schema.Name(set.PackageName)}
	for _, pkg := range set.AdditionalServers {
		serverPackages = append(serverPackages, schema.Name(pkg))
	}

	focusServers := provision.RequireServers(env, serverPackages...)

	// Changing the graph is fairly heavy-weight at this point, as it will lead to
	// a rebuild of all packages (although they'll likely hit the cache), as well
	// as a full re-deployment, re-port forward, etc. Ideally this would be more
	// incremental by having narrower dependencies. E.g. single server would have
	// a single build, single deployment, etc. And changes to siblings servers
	// would only impact themselves, not all servers. #362
	return compute.Continuously(ctx, &buildAndDeploy{
		obs:            obs,
		env:            env,
		serverPackages: serverPackages,
		focusServers:   focusServers,
	})
}

type buildAndDeploy struct {
	obs            *stackState
	env            provision.Env
	serverPackages []schema.PackageName
	focusServers   compute.Computable[*provision.ServerSnapshot]

	mu            sync.Mutex
	cancelRunning func()
}

func (do *buildAndDeploy) Inputs() *compute.In {
	return compute.Inputs().Computable("focusServers", do.focusServers)
}

func (do *buildAndDeploy) Updated(ctx context.Context, r compute.Resolved) error {
	do.mu.Lock()
	defer do.mu.Unlock()

	// If a previous run is on-going, cancel it.
	if do.cancelRunning != nil {
		do.cancelRunning()
		do.cancelRunning = nil
	}

	focusServers := compute.GetDepValue(r, do.focusServers, "focusServers")
	focus, err := focusServers.Get(do.serverPackages...)
	if err != nil {
		return err
	}

	do.obs.updateStack(func(_ *Stack) *Stack {
		// XXX We pass focus[0] as the focus as that's the model the web ui supports right now.
		return computeFirstStack(do.env, focus[0])
	})

	switch do.env.Purpose() {
	case schema.Environment_DEVELOPMENT, schema.Environment_TESTING:
		var observers []languages.DevObserver

		defer func() {
			for _, obs := range observers {
				obs.Close()
			}
		}()

		if do.env.Purpose() == schema.Environment_DEVELOPMENT {
			for _, f := range focus {
				var observer languages.DevObserver

				// Must be invoked before building to make sure stack computation and building
				// uses the updated context.
				ctx, observer, err = languages.IntegrationFor(f.Framework()).PrepareDev(ctx, f)
				if err != nil {
					return err
				}
				if observer != nil {
					observers = append(observers, observer)
				}
			}
		}

		stack, err := stack.Compute(ctx, focus, stack.ProvisionOpts{PortBase: 40000})
		if err != nil {
			return err
		}

		do.obs.updateStack(func(s *Stack) *Stack {
			s.Stack = stack.Proto()
			return s
		})

		plan, err := deploy.PrepareDeployStack(ctx, do.env, stack, focus)
		if err != nil {
			return err
		}

		// The actual build + deploy is deferred into a separate Continuously() call, which
		// reacts to changes to the dependencies of build/deploy (e.g. sources). We can't
		// block here either or else we won't have updates to the package graph to be
		// delivered (Continuously doesn't call Updated, until the previous call returns).

		// A channel is used to signal that the child Continuously() has returned, and
		// thus we can be sure that it's Cleanup has been called (e.g. port forwards
		// have been cancelled, etc).
		done := make(chan struct{})
		cancel := compute.SpawnCancelableOnContinuously(ctx, func(ctx context.Context) error {
			defer close(done)
			return compute.Continuously(ctx, &updateCluster{
				obs:       do.obs,
				localAddr: do.obs.parent.localHostname,
				env:       focusServers.Env(),
				stack:     stack.Proto(),
				focus:     do.serverPackages,
				plan:      plan,
				observers: observers,
			})
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
		if err := tasks.Action(runtime.TaskGraphCompute).Scope(server.PackageName()).Run(ctx,
			func(ctx context.Context) error {
				buildID, err := runtime.For(ctx, do.env).DeployedConfigImageID(ctx, server.Proto())
				if err != nil {
					return err
				}

				s, err := config.Rehydrate(ctx, server, buildID)
				if err != nil {
					return err
				}

				do.obs.updateStack(func(stack *Stack) *Stack {
					stack.Stack = s
					return stack
				})

				return nil
			}); err != nil {
			return err
		}
	}

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

func computeFirstStack(env provision.Env, t provision.Server) *Stack {
	workspace := proto.Clone(env.Root().Workspace).(*schema.Workspace)

	// XXX handling broken web ui builds.
	if workspace.Env == nil {
		workspace.Env = provision.EnvsOrDefault(workspace)
	}

	return &Stack{
		AbsRoot:      env.Root().Abs(),
		Env:          env.Proto(),
		Workspace:    workspace,
		AvailableEnv: workspace.Env,
		Current:      t.StackEntry(),
	}
}
