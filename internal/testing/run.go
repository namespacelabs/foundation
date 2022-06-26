// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const TestRunAction = "test.run"

var errTestFailed = errors.New("test failed")

type testRun struct {
	Env workspace.WorkspaceEnvironment // Doesn't affect the output.

	TestName       string
	TestBinPkg     schema.PackageName
	TestBinCommand []string
	TestBinImageID compute.Computable[oci.ImageID]

	Workspace        *schema.Workspace
	EnvProto         *schema.Environment
	Stack            *schema.Stack
	ServersUnderTest []string // Package names.
	Plan             compute.Computable[*deploy.Plan]
	Debug            bool
	OutputProgress   bool

	// If VClusters are enabled.
	VCluster compute.Computable[*vcluster.VCluster]

	compute.LocalScoped[*PreStoredTestBundle]
}

var _ compute.Computable[*PreStoredTestBundle] = &testRun{}

func (test *testRun) Action() *tasks.ActionEvent {
	return tasks.Action("test").Arg("name", test.TestName).Arg("package_name", test.TestBinPkg)
}

func (test *testRun) Inputs() *compute.In {
	in := compute.Inputs().
		Str("testName", test.TestName).
		Stringer("testBinPkg", test.TestBinPkg).
		Strs("testBinCommand", test.TestBinCommand).
		Computable("testBin", test.TestBinImageID).
		Proto("workspace", test.Workspace).
		Proto("env", test.EnvProto).
		Proto("stack", test.Stack).
		Strs("focus", test.ServersUnderTest).
		Computable("plan", test.Plan).
		Bool("debug", test.Debug)
	if test.VCluster != nil {
		return in.Computable("vcluster", test.VCluster)
	}

	return in
}

func (test *testRun) prepareDeployEnv(ctx context.Context, r compute.Resolved) (ops.Environment, func(context.Context) error, error) {
	if test.VCluster != nil {
		return envWithVCluster(ctx, test.Env, compute.MustGetDepValue(r, test.VCluster, "vcluster"))
	}

	return test.Env, makeDeleteEnv(test.Env), nil
}

func (test *testRun) Compute(ctx context.Context, r compute.Resolved) (*PreStoredTestBundle, error) {
	// The actual test run is wrapped in another action, so we can apply policies to it (e.g. constrain how many tests are deployed in parallel).
	return tasks.Return(ctx, tasks.Action(TestRunAction), func(ctx context.Context) (*PreStoredTestBundle, error) {
		return test.compute(ctx, r)
	})
}

func (test *testRun) compute(ctx context.Context, r compute.Resolved) (*PreStoredTestBundle, error) {
	p := compute.MustGetDepValue(r, test.Plan, "plan")

	env, cleanup, err := test.prepareDeployEnv(ctx, r)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := cleanup(ctx); err != nil {
			fmt.Fprintln(console.Errors(ctx), "Failed to cleanup: ", err)
		}
	}()

	waiters, err := p.Deployer.Execute(ctx, runtime.TaskServerDeploy, env)
	if err != nil {
		return nil, err
	}

	rt := runtime.For(ctx, env)

	if test.OutputProgress {
		fmt.Fprintf(console.Stderr(ctx), "%s: Test %s\n", test.TestBinPkg, aec.LightBlackF.Apply("RUNNING"))

		if err := deploy.Wait(ctx, env, waiters); err != nil {
			var e runtime.ErrContainerFailed
			if errors.As(err, &e) {
				// Don't spend more than N time waiting for logs.
				ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()

				for k, failed := range e.FailedContainers {
					out := console.TypedOutput(ctx, fmt.Sprintf("%s:%d", e.Name, k), console.CatOutputTool)
					if err := rt.FetchLogsTo(ctx, out, failed, runtime.FetchLogsOpts{TailLines: 50}); err != nil {
						fmt.Fprintf(console.Warnings(ctx), "failed to retrieve logs of %s: %v\n", e.Name, err)
					}
				}
			}

			return nil, err
		}
	} else {
		// We call ops.WaitMultiple directly here to skip visual output from deploy.Wait.
		if err := ops.WaitMultiple(ctx, waiters, nil); err != nil {
			return nil, err
		}
	}

	testRun := runtime.ServerRunOpts{
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

	ex, wait := executor.Newf(localCtx, "testing.run(%s)", test.TestName)

	var extraOutput []io.Writer
	if test.OutputProgress {
		extraOutput = append(extraOutput, console.Output(ctx, "testlog"))
	}
	testLog, testLogBuf := makeLog(extraOutput...)

	ex.Go(func(ctx context.Context) error {
		defer cancelAll() // When the test is done, cancel logging.

		if err := rt.RunOneShot(ctx, test.TestBinPkg, testRun, testLog); err != nil {
			// XXX consolidate these two.
			var e1 runtime.ErrContainerExitStatus
			var e2 runtime.ErrContainerFailed
			if errors.As(err, &e1) && e1.ExitCode > 0 {
				return errTestFailed
			} else if errors.As(err, &e2) {
				return errTestFailed
			} else {
				return err
			}
		}

		return nil
	})

	testResults := &schema.TestResult{
		Plan:                   deploy.Serialize(test.Workspace, test.EnvProto, test.Stack, p, test.ServersUnderTest),
		ComputedConfigurations: p.Computed,
	}

	// Clear the hints, no point storing those.
	testResults.Plan.Hints = nil

	waitErr := wait()
	if waitErr == nil {
		testResults.Success = true
	} else if errors.Is(waitErr, errTestFailed) {
		testResults.Success = false
	} else {
		return nil, waitErr
	}

	if test.OutputProgress {
		fmt.Fprintln(console.Stdout(ctx), "Collecting post-execution server logs...")
	}

	bundle, err := collectLogs(ctx, env, test.Stack, test.ServersUnderTest, test.OutputProgress)
	if err != nil {
		return nil, err
	}

	bundle.Result = testResults
	bundle.TestLog = &Log{
		PackageName: test.TestBinPkg.String(),
		Output:      testLogBuf.Seal().Bytes(),
	}

	return bundle, nil
}

func collectLogs(ctx context.Context, env ops.Environment, stack *schema.Stack, focus []string, printLogs bool) (*PreStoredTestBundle, error) {
	ex, wait := executor.New(ctx, "test.collect-logs")

	type serverLog struct {
		PackageName   string
		ContainerName string
		ContainerKind schema.ContainerKind
		Buffer        *syncbuffer.ByteBuffer
	}

	var serverLogs []serverLog
	var mu sync.Mutex // Protects concurrent access to serverLogs.

	for _, entry := range stack.Entry {
		srv := entry.Server // Close on srv.

		if srv.ClusterAdmin {
			// Skip logs for admin servers.
			continue
		}

		rt := runtime.For(ctx, env)

		ex.Go(func(ctx context.Context) error {
			containers, err := rt.ResolveContainers(ctx, srv)
			if err != nil {
				return err
			}

			for _, ctr := range containers {
				ctr := ctr // Close on ctr.

				var extraOutput []io.Writer
				if printLogs && slices.Contains(focus, srv.PackageName) {
					name := srv.Name
					if len(containers) > 0 {
						name = ctr.HumanReference()
					}
					extraOutput = append(extraOutput, console.Output(ctx, name))
				}

				w, log := makeLog(extraOutput...)

				mu.Lock()
				serverLogs = append(serverLogs, serverLog{
					PackageName:   srv.PackageName,
					ContainerName: ctr.HumanReference(),
					ContainerKind: ctr.Kind(),
					Buffer:        log,
				})
				mu.Unlock()

				ex.Go(func(ctx context.Context) error {
					err := runtime.For(ctx, env).FetchLogsTo(ctx, w, ctr, runtime.FetchLogsOpts{IncludeTimestamps: true})
					if errors.Is(err, context.Canceled) {
						return nil
					}
					return err
				})
			}

			return nil
		})
	}

	if err := wait(); err != nil {
		return nil, err
	}

	bundle := &PreStoredTestBundle{}

	for _, entry := range serverLogs {
		bundle.ServerLog = append(bundle.ServerLog, &Log{
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
