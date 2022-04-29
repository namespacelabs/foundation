// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/syncbuffer"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var errTestFailed = errors.New("test failed")

type testRun struct {
	Env workspace.WorkspaceEnvironment // Doesn't affect the output.

	TestName       string
	TestBinPkg     schema.PackageName
	TestBinCommand []string
	TestBinImageID compute.Computable[oci.ImageID]

	Stack *schema.Stack
	Focus []string // Package names.
	Plan  compute.Computable[*deploy.Plan]
	Debug bool

	compute.LocalScoped[*TestBundle]
}

var _ compute.Computable[*TestBundle] = &testRun{}

func (test *testRun) Action() *tasks.ActionEvent {
	return tasks.Action("test").Arg("name", test.TestName).Arg("package_name", test.TestBinPkg)
}

func (test *testRun) Inputs() *compute.In {
	return compute.Inputs().
		Str("testName", test.TestName).
		Stringer("testBinPkg", test.TestBinPkg).
		Strs("testBinCommand", test.TestBinCommand).
		Computable("testBin", test.TestBinImageID).
		Proto("stack", test.Stack).
		Strs("focus", test.Focus).
		Computable("plan", test.Plan).
		Bool("debug", test.Debug)
}

func (test *testRun) Compute(ctx context.Context, r compute.Resolved) (*TestBundle, error) {
	p := compute.GetDepValue(r, test.Plan, "plan")

	waiters, err := p.Deployer.Apply(ctx, runtime.TaskServerDeploy, test.Env)
	if err != nil {
		return nil, err
	}

	var focusServers []*schema.Server
	for _, focus := range test.Focus {
		entry := test.Stack.GetServer(schema.PackageName(focus))
		if entry == nil {
			return nil, fnerrors.InternalError("%s: not present in stack?", focus)
		}
		focusServers = append(focusServers, entry.Server)
	}

	rt := runtime.For(ctx, test.Env)

	if err := deploy.Wait(ctx, test.Env, focusServers, waiters); err != nil {
		var e runtime.ErrContainerFailedToStart
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

	testRun := runtime.ServerRunOpts{
		Image:              compute.GetDepValue(r, test.TestBinImageID, "testBin"),
		Command:            test.TestBinCommand,
		Args:               nil,
		ReadOnlyFilesystem: true,
	}

	if test.Debug {
		testRun.Args = append(testRun.Args, "--debug")
	}

	localCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	ex, wait := executor.New(localCtx)

	testLog, testLogBuf := makeLog(ctx, "testlog", true)

	ex.Go(func(ctx context.Context) error {
		defer cancelAll() // When the test is done, cancel logging.

		if err := rt.RunOneShot(ctx, test.TestBinPkg, testRun, testLog); err != nil {
			var e runtime.ErrContainerExitStatus
			if errors.As(err, &e) && e.ExitCode > 0 {
				return errTestFailed
			} else {
				return err
			}
		}

		return nil
	})

	testResults := &TestResult{}

	waitErr := wait()
	if waitErr == nil {
		testResults.Success = true
	} else if errors.Is(waitErr, errTestFailed) {
		testResults.Success = false
	} else {
		return nil, waitErr
	}

	fmt.Fprintln(console.Stdout(ctx), "Collecting post-execution server logs...")
	bundle, err := collectLogs(ctx, test.Env, test.Stack, test.Focus)
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

func collectLogs(ctx context.Context, env ops.Environment, stack *schema.Stack, focus []string) (*TestBundle, error) {
	ex, wait := executor.New(ctx)

	var serverLogs []*syncbuffer.ByteBuffer // Follows same indexing as rt.Focus.

	for _, entry := range stack.Entry {
		srv := entry.Server // Close on srv.

		w, serverLog := makeLog(ctx, srv.Name, slices.Contains(focus, srv.PackageName))
		serverLogs = append(serverLogs, serverLog)

		ex.Go(func(ctx context.Context) error {
			err := runtime.For(ctx, env).StreamLogsTo(ctx, w, srv, runtime.StreamLogsOpts{})
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		})
	}

	if err := wait(); err != nil {
		return nil, err
	}

	bundle := &TestBundle{}

	for k, entry := range stack.Entry {
		bundle.ServerLog = append(bundle.ServerLog, &Log{
			PackageName: entry.GetPackageName().String(),
			Output:      serverLogs[k].Seal().Bytes(),
		})
	}

	return bundle, nil
}

func makeLog(ctx context.Context, name string, focus bool) (io.Writer, *syncbuffer.ByteBuffer) {
	buf := syncbuffer.NewByteBuffer()
	if !focus {
		return buf, buf
	}

	w := io.MultiWriter(console.Output(ctx, name), buf)
	return w, buf
}
