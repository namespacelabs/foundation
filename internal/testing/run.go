// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/morikuni/aec"
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

const TestRunAction = "test.run"

var errTestFailed = errors.New("test failed")

type testRun struct {
	SealedContext cfg.Context     // Doesn't affect the output.
	Cluster       runtime.Cluster // Target, doesn't affect the output.

	TestRef *schema.PackageRef

	TestBinCommand []string
	TestBinImageID compute.Computable[oci.ImageID]

	Stack            *schema.Stack
	ServersUnderTest []string // Package names.
	Plan             compute.Computable[*deploy.Plan]
	Debug            bool
	OutputProgress   bool
	RuntimeConfig    *runtimepb.RuntimeConfig

	compute.LocalScoped[*storage.TestResultBundle]
}

var _ compute.Computable[*storage.TestResultBundle] = &testRun{}

func (test *testRun) Action() *tasks.ActionEvent {
	return tasks.Action("test").Arg("name", test.TestRef.Name).Arg("package_name", test.TestRef.AsPackageName())
}

func (test *testRun) Inputs() *compute.In {
	in := compute.Inputs().
		Str("testName", test.TestRef.Name).
		Stringer("testPkg", test.TestRef.AsPackageName()).
		Strs("testBinCommand", test.TestBinCommand).
		Computable("testBin", test.TestBinImageID).
		Proto("workspace", test.SealedContext.Workspace().Proto()).
		Proto("env", test.SealedContext.Environment()).
		Proto("stack", test.Stack).
		Strs("focus", test.ServersUnderTest).
		Computable("plan", test.Plan).
		Bool("debug", test.Debug)

	return in
}

func (test *testRun) Compute(ctx context.Context, r compute.Resolved) (*storage.TestResultBundle, error) {
	// The actual test run is wrapped in another action, so we can apply policies to it (e.g. constrain how many tests are deployed in parallel).
	return tasks.Return(ctx, tasks.Action(TestRunAction), func(ctx context.Context) (*storage.TestResultBundle, error) {
		return test.compute(ctx, r)
	})
}

func (test *testRun) compute(ctx context.Context, r compute.Resolved) (*storage.TestResultBundle, error) {
	p := compute.MustGetDepValue(r, test.Plan, "plan")

	env := test.SealedContext
	cluster, err := test.Cluster.Bind(test.SealedContext)
	if err != nil {
		return nil, err
	}

	defer func() {
		if !test.SealedContext.Environment().Ephemeral {
			// skip cleanup for non-ephemeral environments (e.g. to allow manual inspection of the resources)
			return
		}
		if _, err := cluster.DeleteRecursively(ctx, false); err != nil {
			fmt.Fprintln(console.Errors(ctx), "Failed to cleanup: ", err)
		}
	}()

	deployPlan := deploy.Serialize(env.Workspace().Proto(), env.Environment(), test.Stack, p, test.ServersUnderTest)

	fmt.Fprintf(console.Stderr(ctx), "%s: Test %s\n", test.TestRef.Canonical(), aec.LightBlackF.Apply("RUNNING"))

	var waitErr error
	if err := orchestration.Deploy(ctx, env, cluster, deployPlan, true, test.OutputProgress); err != nil {
		waitErr = fnerrors.Wrap(test.TestRef.AsPackageName(), err)
	}

	var testLogBuf *syncbuffer.ByteBuffer
	if waitErr == nil {
		// All servers deployed. Lets start the test driver.

		testRun := runtime.ContainerRunOpts{
			Image:              compute.MustGetDepValue(r, test.TestBinImageID, "testBin"),
			Command:            test.TestBinCommand,
			Args:               nil,
			ReadOnlyFilesystem: true,
		}

		if test.Debug {
			testRun.Args = append(testRun.Args, "--debug")
		}

		localCtx, cancelAll := context.WithCancel(ctx)
		defer cancelAll()

		ex := executor.Newf(localCtx, "testing.run(%s)", test.TestRef.Canonical())

		var extraOutput []io.Writer
		if test.OutputProgress {
			extraOutput = append(extraOutput, console.Output(ctx, "testlog"))
		}

		var testLog io.Writer
		testLog, testLogBuf = makeLog(extraOutput...)

		ex.Go(func(ctx context.Context) error {
			defer cancelAll() // When the test is done, cancel logging.

			pkgId := naming.StableIDN(test.TestRef.PackageName, 8)

			testDriver := runtime.DeployableSpec{
				ErrorLocation:          test.TestRef.AsPackageName(),
				PackageName:            test.TestRef.AsPackageName(),
				Class:                  schema.DeployableClass_ONESHOT,
				Id:                     ids.NewRandomBase32ID(8),
				Name:                   fmt.Sprintf("%s-%s", test.TestRef.Name, pkgId),
				MainContainer:          testRun,
				RuntimeConfig:          test.RuntimeConfig,
				MountRuntimeConfigPath: constants.NamespaceConfigMount,
			}

			plan, err := cluster.Planner().PlanDeployment(ctx, runtime.DeploymentSpec{Specs: []runtime.DeployableSpec{testDriver}})
			if err != nil {
				return err
			}

			g := execution.NewPlan(plan.Definitions...)

			// Make sure that the cluster is accessible to a serialized invocation implementation.
			if err := execution.Execute(ctx, env, "test.driver.deploy", g, nil, runtime.InjectCluster(cluster)...); err != nil {
				return fnerrors.New("failed to deploy: %w", err)
			}

			containers, err := cluster.WaitForTermination(ctx, testDriver)
			if err != nil {
				return err
			}

			if len(containers) != 1 {
				return fnerrors.InternalError("expected test driver to yield exactly one container, got %d", len(containers))
			}

			for _, container := range containers {
				if err := cluster.Cluster().FetchLogsTo(ctx, testLog, container.Reference, runtime.FetchLogsOpts{}); err != nil {
					if errors.Is(err, context.Canceled) {
						return err
					}

					fmt.Fprintf(console.Errors(ctx), "%s: failed to fetch test log: %v\n", test.TestRef.Canonical(), err)
				}
			}

			for _, container := range containers {
				// XXX consolidate these two.
				var e1 runtime.ErrContainerExitStatus
				var e2 runtime.ErrContainerFailed
				if errors.As(container.TerminationError, &e1) && e1.ExitCode > 0 {
					return errTestFailed
				} else if errors.As(err, &e2) {
					return errTestFailed
				} else if container.TerminationError != nil {
					return container.TerminationError
				}
			}

			return nil
		})

		waitErr = ex.Wait()
	}

	testResults := &storage.TestResult{}
	if waitErr == nil {
		testResults.Success = true
	} else if errors.Is(waitErr, errTestFailed) {
		testResults.Success = false
	} else {
		testResults.Success = false
		st := status.Convert(waitErr)
		testResults.ErrorCode = int32(st.Code())
		testResults.ErrorMessage = st.Message()
	}

	if test.OutputProgress {
		fmt.Fprintln(console.Stdout(ctx), "Collecting post-execution server logs...")
	}

	// Limit how much time we spend on collecting logs out of the test environment.
	collectionCtx, collectionDone := context.WithTimeout(ctx, 60*time.Second)
	defer collectionDone()

	bundle, err := collectLogs(collectionCtx, env, cluster, test.TestRef, test.Stack, test.ServersUnderTest, test.OutputProgress)
	if err != nil {
		return nil, err
	}

	bundle.DeployPlan = deployPlan
	bundle.ComputedConfigurations = p.Computed
	// Clear the hints, no point storing those.
	bundle.DeployPlan.Hints = nil

	bundle.Result = testResults
	if testLogBuf != nil {
		bundle.TestLog = &storage.TestResultBundle_InlineLog{
			PackageName: test.TestRef.PackageName,
			Output:      testLogBuf.Seal().Bytes(),
		}
	}

	return bundle, nil
}

func collectLogs(ctx context.Context, env cfg.Context, rt runtime.ClusterNamespace, testRef *schema.PackageRef, stack *schema.Stack, focus []string, printLogs bool) (*storage.TestResultBundle, error) {
	ex := executor.New(ctx, "test.collect-logs")

	type serverLog struct {
		PackageName   string
		ContainerName string
		ContainerKind runtimepb.ContainerKind
		Buffer        *syncbuffer.ByteBuffer
	}

	var serverLogs []serverLog
	var mu sync.Mutex // Protects concurrent access to serverLogs.

	out := console.Output(ctx, "test.collect-logs")

	for _, entry := range stack.Entry {
		srv := entry.Server // Close on srv.

		ex.Go(func(ctx context.Context) error {
			// It should be possible to resolve a container fairly quickly. Make
			// sure we don't get stuck here waiting forever.
			resolveCtx, resolveDone := context.WithTimeout(ctx, 10*time.Second)
			defer resolveDone()

			containers, err := rt.ResolveContainers(resolveCtx, srv)
			if err != nil {
				fmt.Fprintf(out, "%s: failed to resolve containers: %s: %v\n", testRef.Canonical(), srv.PackageName, err)
				return nil
			}

			for _, ctr := range containers {
				ctr := ctr // Close on ctr.

				var extraOutput []io.Writer
				if printLogs && slices.Contains(focus, srv.PackageName) {
					name := srv.Name
					if len(containers) > 0 {
						name = ctr.HumanReference
					}
					extraOutput = append(extraOutput, console.Output(ctx, name))
				}

				w, log := makeLog(extraOutput...)

				mu.Lock()
				serverLogs = append(serverLogs, serverLog{
					PackageName:   srv.PackageName,
					ContainerName: ctr.HumanReference,
					ContainerKind: ctr.Kind,
					Buffer:        log,
				})
				mu.Unlock()

				ex.Go(func(ctx context.Context) error {
					err := rt.Cluster().FetchLogsTo(ctx, w, ctr, runtime.FetchLogsOpts{IncludeTimestamps: true})
					if errors.Is(err, context.Canceled) {
						return nil
					}
					if err != nil {
						fmt.Fprintf(out, "%s: failed to fetch logs: %s: %v\n", testRef.Canonical(), srv.PackageName, err)
					}
					return nil
				})
			}

			return nil
		})
	}

	var diagnostics *storage.EnvironmentDiagnostics
	ex.Go(func(ctx context.Context) error {
		var err error
		diagnostics, err = rt.FetchEnvironmentDiagnostics(ctx)
		if err != nil {
			fmt.Fprintf(console.Warnings(ctx), "Failed to retrieve environment diagnostics: %v\n", err)
		}
		return nil
	})

	if err := ex.Wait(); err != nil {
		return nil, err
	}

	bundle := &storage.TestResultBundle{
		EnvDiagnostics: diagnostics,
	}

	for _, entry := range serverLogs {
		bundle.ServerLog = append(bundle.ServerLog, &storage.TestResultBundle_InlineLog{
			PackageName:   entry.PackageName,
			ContainerName: entry.ContainerName,
			ContainerKind: entry.ContainerKind,
			Output:        entry.Buffer.Seal().Bytes(),
		})
	}

	return bundle, nil
}

func makeLog(otherWriters ...io.Writer) (io.Writer, *syncbuffer.ByteBuffer) {
	buf := syncbuffer.NewByteBuffer()
	if len(otherWriters) == 0 {
		return buf, buf
	}
	w := io.MultiWriter(append(otherWriters, buf)...)
	return w, buf
}
