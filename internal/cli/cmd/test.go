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
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/workspace"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	var (
		env                     planning.Context
		locs                    fncobra.Locations
		testOpts                testing.TestOpts
		includeServers          bool
		parallel                bool
		parallelWork            bool = true
		forceOutputProgress     bool
		rocketShip              bool
		ephemeral               bool = true
		explain                 bool
		uploadResultsToRegistry bool
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
			flags.BoolVar(&includeServers, "include_servers", includeServers, "If true, also include generated server startup-tests.")
			flags.BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
			flags.BoolVar(&parallelWork, "parallel_work", parallelWork, "If true, performs all work in parallel except running the actual test (e.g. builds).")
			flags.BoolVar(&explain, "explain", explain, "If set to true, rather than applying the graph, output an explanation of what would be done.")
			flags.BoolVar(&rocketShip, "rocket_ship", rocketShip, "If set, go full parallel without constraints.")
			flags.BoolVar(&forceOutputProgress, "force_output_progress", forceOutputProgress, "If set to true, always output progress, regardless of whether parallel is set.")
			flags.BoolVar(&uploadResultsToRegistry, "upload_results_to_registry", uploadResultsToRegistry, "If set to true, uploads the test results to the configured registry.")

			_ = flags.MarkHidden("rocket_ship")
			_ = flags.MarkHidden("force_output_progress")
		}).
		With(
			// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
			fncobra.HardcodeEnv(&env, "dev"),
			fncobra.ParseLocations(&locs, &env, fncobra.ParseLocationsOpts{ReturnAllIfNoneSpecified: true})).
		Do(func(originalCtx context.Context) error {
			ctx := prepareContext(originalCtx, parallelWork, rocketShip)

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
			testOpts.OutputProgress = !parallel || forceOutputProgress

			parallelTests := make([]compute.Computable[testing.StoredTestResults], len(testRefs))
			testResults := make([]compute.Computable[oci.ImageID], len(testRefs))
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

						testComp, err := testing.PrepareTest(ctx, pl, buildEnv, testRef, testOpts)
						if err != nil {
							var inc fnerrors.IncompatibleEnvironmentErr
							if errors.As(err, &inc) {
								incompatible[k] = &inc
								if !parallel && !parallelWork {
									printIncompatible(stderr, style, testRef)
								}

								return nil
							}

							return fnerrors.UserError(testRef.AsPackageName(), "failed to prepare test: %w", err)
						}

						if parallel || parallelWork {
							parallelTests[k] = testComp

							if uploadResultsToRegistry {
								testResults[k], err = testing.UploadResults(ctx, buildEnv, testRef.AsPackageName(), testComp)
								if err != nil {
									return fnerrors.InternalError("failed to allocate image tag: %w", err)
								}
							}
						} else {
							if explain {
								return compute.Explain(ctx, console.Stdout(ctx), testComp)
							}

							testResults, err := compute.Get(ctx, testComp)
							if err != nil {
								return err
							}

							runs.Run[k] = &storage.TestRuns_Run{
								TestSummary: testResults.Value.TestBundleSummary,
								TestResults: testResults.Value.Bundle,
							}

							if uploadResultsToRegistry {
								img, err := testing.UploadResults(ctx, buildEnv, testRef.AsPackageName(), testComp)
								if err != nil {
									return fnerrors.InternalError("failed to allocate image tag: %w", err)
								}

								res, err := compute.GetValue(ctx, img)
								if err != nil {
									return err
								}

								runs.Run[k].TestBundleId = res.ImageRef()
							}

							printResult(stderr, style, testRef, runs.Run[k], false)
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

				var resultImages []oci.ImageID
				if uploadResultsToRegistry {
					results, err := compute.GetValue(ctx, compute.Collect(tasks.Action("test.upload-results"), testResults...))
					if err != nil {
						return err
					}

					for _, r := range results {
						resultImages = append(resultImages, r.Value)
					}
				}

				results, err := compute.GetValue(ctx, runTests)
				if err != nil {
					return err
				}

				for k, res := range results {
					if res.Set {
						runs.Run[k] = &storage.TestRuns_Run{
							TestSummary: res.Value.TestBundleSummary,
							TestResults: res.Value.Bundle,
						}
						if uploadResultsToRegistry {
							runs.Run[k].TestBundleId = resultImages[k].ImageRef()
						}
					} else {
						runs.IncompatibleTest = append(runs.IncompatibleTest, &storage.TestRuns_IncompatibleTest{
							TestPackage:       testRefs[k].AsPackageName().String(),
							TestName:          testRefs[k].Name,
							ServerPackage:     incompatible[k].ServerPackageName.String(),
							RequirementOwner:  incompatible[k].RequirementOwner.String(),
							RequiredLabel:     incompatible[k].RequiredLabel,
							IncompatibleLabel: incompatible[k].IncompatibleLabel,
						})
					}
				}

				sortedRuns := slices.Clone(runs.Run)
				// Sorting after processing results since the indices need to be synchronized with other slices.
				slices.SortFunc(sortedRuns, func(a, b *storage.TestRuns_Run) bool {
					return a.TestSummary.Result.Success && !b.TestSummary.Result.Success
				})

				for _, res := range sortedRuns {
					pkgRef := schema.MakePackageRef(schema.PackageName(res.TestSummary.TestPackage), res.TestSummary.TestName)
					printResult(stderr, style, pkgRef, res, true)
				}

				for _, res := range runs.IncompatibleTest {
					pkgRef := schema.MakePackageRef(schema.PackageName(res.TestPackage), res.TestName)
					printIncompatible(stderr, style, pkgRef)
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
