// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	enverr "namespacelabs.dev/foundation/internal/fnerrors/env"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/idtypes"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [path/to/package]...",
		Short: "Run a functional end-to-end test.",
		Args:  cobra.ArbitraryArgs,
	}

	var (
		testOpts            testing.TestOpts
		allTests            bool
		parallel            bool
		parallelWork        bool = true
		forceOutputProgress bool
		rocketShip          bool
		ephemeral           bool = true
		explain             bool
		concurrentTests     int32 = 1
	)

	flags := cmd.Flags()
	flags.BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
	flags.BoolVar(&ephemeral, "ephemeral", ephemeral, "If true, cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
	flags.BoolVar(&allTests, "all", allTests, "If true, runs all tests regardless of tag (e.g. it will also include generated tests).")
	flags.BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
	flags.BoolVar(&parallelWork, "parallel_work", parallelWork, "If true, performs all work in parallel except running the actual test (e.g. builds).")
	flags.BoolVar(&explain, "explain", explain, "If set to true, rather than applying the graph, output an explanation of what would be done.")

	logDir := flags.String("log_dir", "", "If set, write all log files to this directory.")

	flags.BoolVar(&rocketShip, "rocket_ship", rocketShip, "If set, go full parallel without constraints.")
	_ = flags.MarkHidden("rocket_ship")

	flags.BoolVar(&forceOutputProgress, "force_output_progress", forceOutputProgress, "If set to true, always output progress, regardless of whether parallel is set.")
	_ = flags.MarkHidden("force_output_progress")

	flags.Int32Var(&concurrentTests, "concurrent_test_limit", concurrentTests, "Limit how many tests may run in parallel.")
	_ = flags.MarkHidden("concurrent_test_limit")

	// Deprecated.
	flags.Bool("include_servers", false, "Does nothing.")
	_ = flags.MarkHidden("include_servers")

	specificEnv := cmd.Flags().String("source_env", "dev", "Which environment to use as configuration base.")

	env := fncobra.EnvFromValue(cmd, specificEnv)
	locs := fncobra.LocationsFromArgs(cmd, env, fncobra.ParseLocationsOpts{
		ReturnAllIfNoneSpecified: true,
		SupportPackageRef:        true,
	})

	return fncobra.With(cmd, func(originalCtx context.Context) error {
		if parallel && concurrentTests == 1 {
			concurrentTests = 5
		}

		ctx := prepareContext(originalCtx, rocketShip, concurrentTests)

		if rocketShip || concurrentTests > 1 {
			parallel = true
		}

		// This PackageLoader instance is only used to resolve package references from the command line arguments.
		packageRefPl := parsing.NewPackageLoader(*env)

		var testRefs []*schema.PackageRef
		for _, l := range locs.Locations {
			pp, err := packageRefPl.LoadByName(ctx, l.AsPackageName())
			if err != nil {
				return err
			}

			for _, t := range pp.Tests {
				if allTests || !slices.Contains(t.Tag, "generated") {
					testRefs = append(testRefs, schema.MakePackageRef(l.AsPackageName(), t.Name))
				}
			}
		}
		// add package references
		testRefs = append(testRefs, locs.Refs...)

		if len(testRefs) == 0 {
			return noTestsError(ctx, allTests, *locs)
		}

		out := console.TypedOutput(ctx, "test-results", idtypes.CatOutputUs)
		style := colors.Ctx(ctx)

		testOpts.ParentRunID = storedrun.ParentID
		testOpts.OutputProgress = (!parallel || forceOutputProgress) && *logDir == ""

		var mu sync.Mutex // Protects the slices below.
		var pending []compute.Computable[testing.StoredTestResults]
		var incompatible []incompatibleTest
		var completed []*storage.TestRuns_Run

		if err := tasks.Action("test.prepare").Run(ctx, func(ctx context.Context) error {
			eg := executor.New(ctx, "test")

			for _, testRef := range testRefs {
				testRef := testRef // Capture testRef.

				eg.Go(func(ctx context.Context) error {
					buildEnv, err := testing.PrepareEnv(ctx, *env, ephemeral)
					if err != nil {
						return err
					}

					pl := parsing.NewPackageLoader(buildEnv)

					status := style.Header.Apply("BUILDING")
					fmt.Fprintf(out, "%s: Test %s\n", testRef.Canonical(), status)

					testComp, err := testing.PrepareTest(ctx, pl, buildEnv, testRef, testOpts)
					if err != nil {
						var inc enverr.IncompatibleEnvironmentErr
						if errors.As(err, &inc) {
							mu.Lock()
							incompatible = append(incompatible, incompatibleTest{
								Ref: testRef,
								Err: &inc,
							})
							mu.Unlock()

							if !parallel && !parallelWork {
								printIncompatible(out, style, testRef)
							}

							return nil
						}

						return fnerrors.NewWithLocation(testRef.AsPackageName(), "failed to prepare test: %w", err)
					}

					if parallel || parallelWork {
						mu.Lock()
						pending = append(pending, testComp)
						mu.Unlock()
					} else {
						if explain {
							return compute.Explain(ctx, console.Stdout(ctx), testComp)
						}

						testResults, err := compute.Get(ctx, testComp)
						if err != nil {
							return err
						}

						run := &storage.TestRuns_Run{
							TestSummary: testResults.Value.TestBundleSummary,
							TestResults: testResults.Value.Bundle,
						}

						mu.Lock()
						completed = append(completed, run)
						mu.Unlock()

						printResult(out, style, testRef, run, false)
					}

					return nil
				})
			}

			return eg.Wait()
		}); err != nil {
			return err
		}

		if len(pending) > 0 {
			runTests := compute.Collect(tasks.Action("test.all-tests"), pending...)

			if explain {
				return compute.Explain(ctx, console.Stdout(ctx), runTests)
			}

			results, err := compute.GetValue(ctx, runTests)
			if err != nil {
				return err
			}

			for _, res := range results {
				completed = append(completed, &storage.TestRuns_Run{
					TestSummary: res.Value.TestBundleSummary,
					TestResults: res.Value.Bundle,
				})
			}
		}

		// Sorting after processing results since the indices need to be synchronized with other slices.
		slices.SortFunc(completed, func(a, b *storage.TestRuns_Run) int {
			if a.TestSummary.Result.Success && !b.TestSummary.Result.Success {
				return -1
			}

			return 0
		})

		if len(pending) > 0 {
			fmt.Fprintln(out)

			for _, res := range completed {
				pkgRef := schema.MakePackageRef(schema.PackageName(res.TestSummary.TestPackage), res.TestSummary.TestName)
				printResult(out, style, pkgRef, res, true)
			}

			for _, res := range incompatible {
				printIncompatible(out, style, res.Ref)
			}
		}

		var failed []string
		for _, run := range completed {
			if !run.GetTestSummary().GetResult().Success {
				failed = append(failed, run.GetTestSummary().GetTestPackage())
			}
		}

		if *logDir != "" {
			var errs []error
			for k, run := range completed {
				ld := *logDir
				if len(completed) > 1 {
					ld = filepath.Join(ld, fmt.Sprintf("%d", k))
				}

				if err := os.MkdirAll(ld, 0755); err != nil {
					return err
				}

				dumpfile := func(name string, s *storage.TestResultBundle_InlineLog) error {
					fpath := filepath.Join(ld, name)
					f, err := os.Create(fpath)
					if err != nil {
						return err
					}

					defer f.Close()

					if _, err := f.Write(s.Output); err != nil {
						return err
					}

					fmt.Fprintf(out, "Wrote %s\n", fpath)

					return nil
				}

				if log := run.TestResults.GetTestLog(); log != nil {
					errs = append(errs, dumpfile("test.log", log))
				}

				for _, log := range run.TestResults.GetServerLog() {
					errs = append(errs, dumpfile(fmt.Sprintf("%s.%s.log", strings.ReplaceAll(log.PackageName, "/", "--"), log.ContainerName), log))
				}
			}

			if err := multierr.New(errs...); err != nil {
				return err
			}
		}

		runs := &storage.TestRuns{Run: completed}
		storedrun.Attach(runs)

		if len(failed) > 0 {
			return fnerrors.ExitWithCode(fmt.Errorf("failed tests: %s", strings.Join(failed, ", ")), exitCode)
		}

		return nil
	})
}

func prepareContext(ctx context.Context, rocketShip bool, concurrentTests int32) context.Context {
	if rocketShip {
		fmt.Fprintln(console.Stdout(ctx), "Engaging 🚀 mode; all throttling disabled.")
		return tasks.ContextWithThrottler(ctx, console.Debug(ctx), &tasks.ThrottleConfigurations{})
	}

	configs := &tasks.ThrottleConfigurations{}
	configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, tasks.BaseDefaultConfig()...)
	configs.ThrottleConfiguration = append(configs.ThrottleConfiguration, &tasks.ThrottleConfiguration{
		Labels: map[string]string{"action": testing.TestRunAction}, Capacity: int32(concurrentTests),
	})
	return tasks.ContextWithThrottler(ctx, console.Debug(ctx), configs)
}

type incompatibleTest struct {
	Ref *schema.PackageRef
	Err *enverr.IncompatibleEnvironmentErr
}

func printResult(out io.Writer, style colors.Style, testRef *schema.PackageRef, res *storage.TestRuns_Run, printResults bool) {
	status := style.TestSuccess.Apply("PASSED")

	r := res.TestResults.Result
	if !r.Success {
		if printResults {
			for _, srv := range res.TestResults.ServerLog {
				printLog(out, srv)
			}
			if res.TestResults.TestLog != nil {
				printLog(out, res.TestResults.TestLog)
			}
		}

		status = style.TestFailure.Apply("FAILED")

		if r.ErrorMessage != "" {
			status += style.TestFailure.Apply(fmt.Sprintf(" (%s)", r.ErrorMessage))
		}
	}

	var suffix string
	if res.TestSummary.GetCreated() != nil && res.TestSummary.GetStarted() != nil && res.TestSummary.GetCompleted() != nil {
		suffix = style.Comment.Apply(fmt.Sprintf(" (waited %v, took %v)",
			humanizeDuration(res.TestSummary.Started.AsTime().Sub(res.TestSummary.Created.AsTime()), 2),
			humanizeDuration(res.TestSummary.Completed.AsTime().Sub(res.TestSummary.Started.AsTime()), 2)))
	}

	fmt.Fprintf(out, "%s: Test %s%s\n", testRef.Canonical(), status, suffix)
}

func humanizeDuration(duration time.Duration, n int) string {
	days := duration.Hours() / 24
	hours := math.Mod(duration.Hours(), 24)
	minutes := math.Mod(duration.Minutes(), 60)
	seconds := math.Mod(duration.Seconds(), 60)

	chunks := []struct {
		suffix string
		amount float64
	}{
		{"d", days},
		{"h", hours},
		{"m", minutes},
		{"s", seconds},
	}

	var parts []string
	for k, chunk := range chunks {
		if int64(math.Floor(chunk.amount)) == 0 {
			continue
		}

		mod := "%.0f%s"
		if len(parts) == (n-1) || k == len(chunks)-1 {
			mod = "%.1f%s"
		}

		parts = append(parts, fmt.Sprintf(mod, chunk.amount, chunk.suffix))
		if len(parts) == n {
			break
		}
	}

	return strings.Join(parts, "")
}

func printIncompatible(out io.Writer, style colors.Style, testRef *schema.PackageRef) {
	fmt.Fprintf(out, "%s: Test %s\n", testRef.Canonical(), style.LogCachedName.Apply("INCOMPATIBLE"))
}

func printLog(out io.Writer, log *storage.TestResultBundle_InlineLog) {
	for _, line := range bytes.Split(log.Output, []byte("\n")) {
		fmt.Fprintf(out, "%s:%s: %s\n", log.PackageName, log.ContainerName, line)
	}
}

func noTestsError(ctx context.Context, allTests bool, locs fncobra.Locations) error {
	where := "in this workspace"
	if locs.UserSpecified {
		where = "in the specified package"
		if len(locs.Locations) > 1 {
			where += "s"
		}
	}

	if !allTests {
		return fnerrors.Newf("No user-defined tests found %s. Try adding `--all` to include generated tests.", where)
	}

	msg := fmt.Sprintf("No servers to test found %s.", where)

	if !locs.UserSpecified {
		// `ns test --all` should pass on empy repositories.
		fmt.Fprintln(console.Stdout(ctx), msg)
		return nil
	}

	return fnerrors.New(msg)
}
