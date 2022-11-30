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
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	enverr "namespacelabs.dev/foundation/internal/fnerrors/env"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	var (
		env                 cfg.Context
		locs                fncobra.Locations
		testOpts            testing.TestOpts
		allTests            bool
		parallel            bool
		parallelWork        bool = true
		forceOutputProgress bool
		rocketShip          bool
		ephemeral           bool = true
		explain             bool
	)

	return fncobra.
		Cmd(&cobra.Command{
			Use:   "test [path/to/package]...",
			Short: "Run a functional end-to-end test.",
			Args:  cobra.ArbitraryArgs,
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
			flags.BoolVar(&ephemeral, "ephemeral", ephemeral, "If true, cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
			flags.BoolVar(&allTests, "all", allTests, "If true, runs all tests regardless of tag (e.g. it will also include generated tests).")
			flags.BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
			flags.BoolVar(&parallelWork, "parallel_work", parallelWork, "If true, performs all work in parallel except running the actual test (e.g. builds).")
			flags.BoolVar(&explain, "explain", explain, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.BoolVar(&rocketShip, "rocket_ship", rocketShip, "If set, go full parallel without constraints.")
			flags.BoolVar(&forceOutputProgress, "force_output_progress", forceOutputProgress, "If set to true, always output progress, regardless of whether parallel is set.")

			_ = flags.MarkHidden("rocket_ship")
			_ = flags.MarkHidden("force_output_progress")

			// Deprecated.
			flags.Bool("include_servers", false, "Does nothing.")
			_ = flags.MarkHidden("include_servers")
		}).
		With(
			// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true, SupportPackageRef: true})).
		Do(func(originalCtx context.Context) error {
			ctx := prepareContext(originalCtx, parallelWork, rocketShip)

			if rocketShip {
				parallel = true
			}

			// This PackageLoader instance is only used to resolve package references from the command line arguments.
			packageRefPl := parsing.NewPackageLoader(env)

			testRefs := []*schema.PackageRef{}
			for _, l := range locs.Locs {
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
			// add package refernces
			testRefs = append(testRefs, locs.Refs...)

			if len(testRefs) == 0 {
				return noTestsError(ctx, allTests, locs)
			}

			out := console.TypedOutput(ctx, "test-results", common.CatOutputUs)
			style := colors.Ctx(ctx)

			testOpts.ParentRunID = storedrun.ParentID
			testOpts.OutputProgress = !parallel || forceOutputProgress

			var mu sync.Mutex // Protects the slices below.
			var pending []compute.Computable[testing.StoredTestResults]
			var incompatible []incompatibleTest
			var completed []*storage.TestRuns_Run

			if err := tasks.Action("test.prepare").Run(ctx, func(ctx context.Context) error {
				eg := executor.New(ctx, "test")

				for _, testRef := range testRefs {
					testRef := testRef // Capture testRef.

					eg.Go(func(ctx context.Context) error {
						buildEnv, err := testing.PrepareEnv(ctx, env, ephemeral)
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
			slices.SortFunc(completed, func(a, b *storage.TestRuns_Run) bool {
				return a.TestSummary.Result.Success && !b.TestSummary.Result.Success
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

			runs := &storage.TestRuns{Run: completed}
			storedrun.Attach(runs)

			if len(failed) > 0 {
				return fnerrors.ExitWithCode(fmt.Errorf("failed tests: %s", strings.Join(failed, ", ")), exitCode)
			}

			return nil
		})
}

func prepareContext(ctx context.Context, parallelWork, rocketShip bool) context.Context {
	if rocketShip {
		fmt.Fprintln(console.Stdout(ctx), "Engaging 🚀 mode; all throttling disabled.")
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
	if res.TestBundleId != "" {
		suffix = fmt.Sprintf(" %s", style.Comment.Apply(res.TestBundleId))
	}

	fmt.Fprintf(out, "%s: Test %s%s\n", testRef.Canonical(), status, suffix)
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
		if len(locs.Locs) > 1 {
			where += "s"
		}
	}

	if !allTests {
		return fnerrors.New("No user-defined tests found %s. Try adding `--all` to include generated tests.", where)
	}

	msg := fmt.Sprintf("No servers to test found %s.", where)

	if !locs.UserSpecified {
		// `ns test --all` should pass on empy repositories.
		fmt.Fprintln(console.Stdout(ctx), msg)
		return nil
	}

	return fnerrors.New(msg)
}
