// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	var (
		testOpts       testing.TestOpts
		includeServers bool
		parallel       bool
		parallelWork   bool
		ephemeral      bool = true
		explain        bool
	)

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run a functional end-to-end test.",
		Args:  cobra.ArbitraryArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			root, err := module.FindRoot(ctx, ".")
			if err != nil {
				return err
			}

			devEnv, err := provision.RequireEnv(root, "dev")
			if err != nil {
				return err
			}

			var locs []fnfs.Location

			if len(args) == 0 {
				list, err := workspace.ListSchemas(ctx, root)
				if err != nil {
					return err
				}

				pl := workspace.NewPackageLoader(root)
				for _, l := range list.Locations {
					pp, err := pl.LoadByName(ctx, l.AsPackageName())
					if err != nil {
						return err
					}

					// We also automatically generate a startup-test for each server.
					if pp.Test != nil || (includeServers && pp.Server != nil) {
						locs = append(locs, l)
					}
				}
			} else {
				for _, arg := range args {
					loc := root.RelPackage(arg)
					locs = append(locs, loc)
				}
			}

			stderr := console.Stderr(ctx)
			pl := workspace.NewPackageLoader(devEnv.Root())

			testOpts.ParentRunID = storedrun.ParentID
			testOpts.OutputProgress = !parallel

			style := colors.Ctx(ctx)

			parallelTests := make([]compute.Computable[testing.StoredTestResults], len(locs))
			runs := &storage.TestRuns{Run: make([]*storage.TestRuns_Run, len(locs))}

			if err := tasks.Action("test.prepare").Run(ctx, func(ctx context.Context) error {
				eg := executor.New(ctx, "test")
				for k, loc := range locs {
					k := k     // Capture k.
					loc := loc // Capture loc.

					eg.Go(func(ctx context.Context) error {
						// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
						buildEnv := testing.PrepareEnv(ctx, devEnv, ephemeral)

						status := style.Header.Apply("BUILDING")
						fmt.Fprintf(stderr, "%s: Test %s\n", loc.AsPackageName(), status)

						test, err := testing.PrepareTest(ctx, pl, buildEnv, loc.AsPackageName(), testOpts, func(ctx context.Context, pl *workspace.PackageLoader, test *schema.Test) ([]provision.Server, *stack.Stack, error) {
							var suts []provision.Server

							for _, pkg := range test.ServersUnderTest {
								sut, err := buildEnv.RequireServerWith(ctx, pl, schema.PackageName(pkg))
								if err != nil {
									return nil, nil, err
								}
								suts = append(suts, sut)
							}

							stack, err := stack.Compute(ctx, suts, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
							if err != nil {
								return nil, nil, err
							}

							return suts, stack, nil
						})
						if err != nil {
							return fnerrors.UserError(loc, "failed to prepare test: %w", err)
						}

						if parallel || parallelWork {
							parallelTests[k] = test
						} else {
							if explain {
								return compute.Explain(ctx, console.Stdout(ctx), test)
							}

							testResults, err := compute.Get(ctx, test)
							if err != nil {
								return err
							}

							printResult(stderr, style, testResults, false)

							runs.Run[k] = &storage.TestRuns_Run{
								TestBundleId: testResults.Value.ImageRef.ImageRef(),
								TestSummary:  testResults.Value.TestBundleSummary,
							}
						}

						return nil
					})
				}

				return eg.Wait()
			}); err != nil {
				return err
			}

			testCtx := ctx
			if parallelWork {
				configs := &tasks.ThrottleConfigurations{}
				configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, tasks.BaseDefaultConfig...)
				configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, &tasks.ThrottleConfiguration{
					Labels: map[string]string{"action": testing.TestRunAction}, Capacity: 1,
				})
				testCtx = tasks.ContextWithThrottler(testCtx, console.Debug(testCtx), configs)
			}

			if len(parallelTests) > 0 {
				runTests := compute.Collect(tasks.Action("test.all-tests"), parallelTests...)

				if explain {
					return compute.Explain(ctx, console.Stdout(ctx), runTests)
				}

				results, err := compute.GetValue(testCtx, runTests)
				if err != nil {
					return err
				}

				for k, res := range results {
					printResult(stderr, style, res, true)

					runs.Run[k] = &storage.TestRuns_Run{
						TestBundleId: res.Value.ImageRef.ImageRef(),
						TestSummary:  res.Value.TestBundleSummary,
					}
				}
			}

			var failed []string
			for _, run := range runs.Run {
				if !run.GetTestSummary().GetResult().Success {
					failed = append(failed, run.GetTestSummary().GetTestPackage())
				}
			}

			storedrun.Attach(runs)

			if len(failed) > 0 {
				return fnerrors.ExitWithCode(fmt.Errorf("failed tests: %s", strings.Join(failed, ", ")), exitCode)
			}

			return nil
		}),
	}

	cmd.Flags().BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", ephemeral, "If true, don't cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
	cmd.Flags().BoolVar(&includeServers, "include_servers", includeServers, "If true, also include generated server startup-tests.")
	cmd.Flags().BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
	cmd.Flags().BoolVar(&parallelWork, "parallel_work", true, "If true, performs all work in parallel except running the actual test (e.g. builds).")
	cmd.Flags().BoolVar(&testing.UseVClusters, "vcluster", testing.UseVClusters, "If true, creates a separate vcluster per test invocation.")
	cmd.Flags().BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")

	return cmd
}

func printResult(out io.Writer, style colors.Style, res compute.ResultWithTimestamp[testing.StoredTestResults], printResults bool) {
	status := style.TestSuccess.Apply("PASSED")
	if !res.Value.Bundle.Result.Success {
		if printResults {
			for _, srv := range res.Value.Bundle.ServerLog {
				printLog(out, srv)
			}
			if res.Value.Bundle.TestLog != nil {
				printLog(out, res.Value.Bundle.TestLog)
			}
		}

		status = style.TestFailure.Apply("FAILED")

		if res.Value.Bundle.Result.ErrorMessage != "" {
			status += style.TestFailure.Apply(fmt.Sprintf(" (%s)", res.Value.Bundle.Result.ErrorMessage))
		}
	}

	cached := ""
	if res.Cached {
		cached = style.LogCachedName.Apply(" (CACHED)")
	}

	fmt.Fprintf(out, "%s: Test %s%s %s\n", res.Value.Package, status, cached, style.Comment.Apply(res.Value.ImageRef.ImageRef()))
}

func printLog(out io.Writer, log *storage.TestResultBundle_InlineLog) {
	for _, line := range bytes.Split(log.Output, []byte("\n")) {
		fmt.Fprintf(out, "%s:%s: %s\n", log.PackageName, log.ContainerName, line)
	}
}
