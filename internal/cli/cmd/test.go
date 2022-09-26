// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	var (
		env            planning.Context
		locs           fncobra.Locations
		testOpts       testing.TestOpts
		includeServers bool
		parallel       bool
		parallelWork   bool = true
		rocketShip     bool
		ephemeral      bool = true
		explain        bool
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "test [path/to/package]...",
			Short: "Run a functional end-to-end test.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
			flags.BoolVar(&ephemeral, "ephemeral", ephemeral, "If true, don't cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
			flags.BoolVar(&includeServers, "include_servers", includeServers, "If true, also include generated server startup-tests.")
			flags.BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
			flags.BoolVar(&parallelWork, "parallel_work", parallelWork, "If true, performs all work in parallel except running the actual test (e.g. builds).")
			flags.BoolVar(&explain, "explain", false, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.BoolVar(&rocketShip, "rocket_ship", false, "If set, go full parallel without constraints.")

			_ = flags.MarkHidden("rocket_ship")
		}).
		With(
			// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true})).
		Do(func(ctx context.Context) error {
			ctx = prepareContext(ctx, parallelWork, rocketShip)

			if rocketShip {
				parallel = true
			}

			// This PackageLoader instance is only used to resolve package references from the command line arguments.
			packageRefPl := workspace.NewPackageLoader(env)

			includeStartupTests := includeServers || locs.UserSpecified
			testRefs := []*schema.PackageRef{}
			for _, l := range locs.Locs {
				pp, err := packageRefPl.LoadByName(ctx, l.AsPackageName())
				if err != nil {
					return err
				}

				for _, t := range pp.Tests {
					if includeStartupTests || t.Driver.PackageName != workspace.StartupTestBinary {
						testRefs = append(testRefs, schema.MakePackageRef(l.AsPackageName(), t.Name))
					}
				}
			}

			stderr := console.Stderr(ctx)
			style := colors.Ctx(ctx)

			testOpts.ParentRunID = storedrun.ParentID
			testOpts.OutputProgress = !parallel

			parallelTests := make([]compute.Computable[testing.StoredTestResults], len(testRefs))
			runs := &storage.TestRuns{Run: make([]*storage.TestRuns_Run, len(testRefs))}
			incompatible := make([]*fnerrors.IncompatibleEnvironmentErr, len(testRefs))

			if err := tasks.Action("test.prepare").Run(ctx, func(ctx context.Context) error {
				eg := executor.New(ctx, "test")

				for k, testRef := range testRefs {
					k := k             // Capture k.
					testRef := testRef // Capture testRef.

					eg.Go(func(ctx context.Context) error {
						buildEnv := testing.PrepareEnv(ctx, env, ephemeral)
						pl := workspace.NewPackageLoader(buildEnv)

						status := style.Header.Apply("BUILDING")
						fmt.Fprintf(stderr, "%s: Test %s\n", testRef.Canonical(), status)

						testComp, err := testing.PrepareTest(ctx, pl, buildEnv, testRef, testOpts, func(ctx context.Context, pl *workspace.PackageLoader, test *schema.Test) ([]parsed.Server, *provision.Stack, error) {
							var suts []parsed.Server

							for _, pkg := range test.ServersUnderTest {
								sut, err := parsed.RequireServerWith(ctx, buildEnv, pl, schema.PackageName(pkg))
								if err != nil {
									return nil, nil, err
								}
								suts = append(suts, sut)
							}

							stack, err := provision.ComputeStack(ctx, suts, provision.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
							if err != nil {
								return nil, nil, err
							}

							return suts, stack, nil
						})
						if err != nil {
							var inc fnerrors.IncompatibleEnvironmentErr
							if errors.As(err, &inc) {
								incompatible[k] = &inc
								if !parallel && !parallelWork {
									var noResults compute.ResultWithTimestamp[testing.StoredTestResults]
									printResult(stderr, style, testRef, noResults, false)
								}

								return nil
							}

							return fnerrors.UserError(testRef.AsPackageName(), "failed to prepare test: %w", err)
						}

						if parallel || parallelWork {
							parallelTests[k] = testComp
						} else {
							if explain {
								return compute.Explain(ctx, console.Stdout(ctx), testComp)
							}

							testResults, err := compute.Get(ctx, testComp)
							if err != nil {
								return err
							}

							printResult(stderr, style, testRef, testResults, false)

							runs.Run[k] = &storage.TestRuns_Run{
								TestBundleId: testResults.Value.ImageRef.ImageRef(),
								TestSummary:  testResults.Value.TestBundleSummary,
								TestResults:  testResults.Value.Bundle,
							}
						}

						return nil
					})
				}

				return eg.Wait()
			}); err != nil {
				return err
			}

			if len(parallelTests) > 0 {
				runTests := compute.Collect(tasks.Action("test.all-tests"), parallelTests...)

				if explain {
					return compute.Explain(ctx, console.Stdout(ctx), runTests)
				}

				results, err := compute.GetValue(ctx, runTests)
				if err != nil {
					return err
				}
				slices.SortFunc(results, func(a, b compute.ResultWithTimestamp[testing.StoredTestResults]) bool {
					return a.Value.Bundle.Result.Success && !b.Value.Bundle.Result.Success
				})

				for k, res := range results {
					printResult(stderr, style, testRefs[k], res, true)

					if res.Set {
						runs.Run[k] = &storage.TestRuns_Run{
							TestBundleId: res.Value.ImageRef.ImageRef(),
							TestSummary:  res.Value.TestBundleSummary,
							TestResults:  res.Value.Bundle,
						}
					} else {
						runs.IncompatibleTest = append(runs.IncompatibleTest, &storage.TestRuns_IncompatibleTest{
							TestPackage:       testRefs[k].AsPackageName().String(),
							ServerPackage:     incompatible[k].ServerPackageName.String(),
							RequirementOwner:  incompatible[k].RequirementOwner.String(),
							RequiredLabel:     incompatible[k].RequiredLabel,
							IncompatibleLabel: incompatible[k].IncompatibleLabel,
						})
					}
				}
			}

			var failed []string
			for _, run := range runs.Run {
				if run != nil {
					if !run.GetTestSummary().GetResult().Success {
						failed = append(failed, run.GetTestSummary().GetTestPackage())
					}
				}
			}

			var withoutNils []*storage.TestRuns_Run
			for _, run := range runs.Run {
				if run != nil {
					withoutNils = append(withoutNils, run)
				}
			}
			runs.Run = withoutNils

			storedrun.Attach(runs)

			if len(failed) > 0 {
				return fnerrors.ExitWithCode(fmt.Errorf("failed tests: %s", strings.Join(failed, ", ")), exitCode)
			}

			return nil
		})
}

func prepareContext(ctx context.Context, parallelWork, rocketShip bool) context.Context {
	if rocketShip {
		return tasks.ContextWithThrottler(ctx, console.Debug(ctx), &tasks.ThrottleConfigurations{})
	}

	if parallelWork {
		configs := &tasks.ThrottleConfigurations{}
		configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, tasks.BaseDefaultConfig...)
		configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, &tasks.ThrottleConfiguration{
			Labels: map[string]string{"action": testing.TestRunAction}, Capacity: 1,
		})
		return tasks.ContextWithThrottler(ctx, console.Debug(ctx), configs)
	}

	return ctx
}

func printResult(out io.Writer, style colors.Style, testRef *schema.PackageRef, res compute.ResultWithTimestamp[testing.StoredTestResults], printResults bool) {
	status := style.TestSuccess.Apply("PASSED")
	cached := ""

	if res.Set {
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

		if res.Cached {
			cached = style.LogCachedName.Apply(" (CACHED)")
		}
	} else {
		status = style.LogCachedName.Apply("INCOMPATIBLE")
	}

	fmt.Fprintf(out, "%s: Test %s%s %s\n", testRef.Canonical(), status, cached, style.Comment.Apply(res.Value.ImageRef.ImageRef()))
}

func printLog(out io.Writer, log *storage.TestResultBundle_InlineLog) {
	for _, line := range bytes.Split(log.Output, []byte("\n")) {
		fmt.Fprintf(out, "%s:%s: %s\n", log.PackageName, log.ContainerName, line)
	}
}
