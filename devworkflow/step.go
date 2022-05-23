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
	"namespacelabs.dev/foundation/internal/runtime/endpointfwd"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func setWorkspace(ctx context.Context, env provision.Env, packageName string, additional []string, obs *Session, pfw *endpointfwd.PortForward) error {
	return compute.Do(ctx, func(ctx context.Context) error {
		serverPackages := []schema.PackageName{schema.Name(packageName)}
		for _, pkg := range additional {
			serverPackages = append(serverPackages, schema.Name(pkg))
		}

		focusServers := provision.RequireServers(env, serverPackages...)

		fmt.Fprintf(console.Debug(ctx), "devworkflow: setWorkspace.Do\n")

		// Changing the graph is fairly heavy-weight at this point, as it will lead to
		// a rebuild of all packages (although they'll likely hit the cache), as well
		// as a full re-deployment, re-port forward, etc. Ideally this would be more
		// incremental by having narrower dependencies. E.g. single server would have
		// a single build, single deployment, etc. And changes to siblings servers
		// would only impact themselves, not all servers. #362
		return compute.Continuously(ctx, &buildAndDeploy{
			obs:            obs,
			pfw:            pfw,
			env:            env,
			serverPackages: serverPackages,
			focusServers:   focusServers,
		}, nil)
	})
}

type buildAndDeploy struct {
	obs            *Session
	pfw            *endpointfwd.PortForward
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
	fmt.Fprintf(console.Debug(ctx), "devworkflow: buildAndDeploy.Updated\n")

	do.mu.Lock()
	defer do.mu.Unlock()

	// If a previous run is on-going, cancel it.
	if do.cancelRunning != nil {
		do.cancelRunning()
		do.cancelRunning = nil
	}

	focusServers := compute.MustGetDepValue(r, do.focusServers, "focusServers")
	focus, err := focusServers.Get(do.serverPackages...)
	if err != nil {
		return err
	}

	do.obs.updateStackInPlace(func(stack *Stack) {
		// XXX We pass focus[0] as the focus as that's the model the web ui supports right now.
		setFirstStack(stack, do.env, focus[0])
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

		do.obs.updateStackInPlace(func(s *Stack) {
			s.Stack = stack.Proto()
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
		// thus we can be sure that its Cleanup has been called.
		done := make(chan struct{})
		transformError := func(err error) error {
			if err != nil {
				if msg, ok := fnerrors.IsExpected(err); ok {
					fmt.Fprintf(console.Stderr(ctx), "\n  %s\n\n", msg)
					return nil
				}
			}
			return err
		}
		cancel := compute.SpawnCancelableOnContinuously(ctx, func(ctx context.Context) error {
			defer close(done)
			return compute.Continuously(ctx,
				newUpdateCluster(focusServers.Env(), stack.Proto(), do.serverPackages, observers, plan, do.pfw),
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

				do.obs.updateStackInPlace(func(stack *Stack) {
					stack.Stack = s.Stack
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

func setFirstStack(out *Stack, env provision.Env, t provision.Server) {
	workspace := proto.Clone(env.Root().Workspace).(*schema.Workspace)

	// XXX handling broken web ui builds.
	if workspace.Env == nil {
		workspace.Env = provision.EnvsOrDefault(workspace)
	}

	out.AbsRoot = env.Root().Abs()
	out.Env = env.Proto()
	out.Workspace = workspace
	out.AvailableEnv = workspace.Env
	out.Current = t.StackEntry()
}
