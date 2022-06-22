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

	"github.com/morikuni/aec"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const exitCode = 3

func NewTestCmd() *cobra.Command {
	var (
		runOpts        deploy.Opts
		testOpts       testing.TestOpts
		includeServers bool
		parallel       bool
		parallelWork   bool
		ephemeral      bool = true
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

			var parallelTests []compute.Computable[testing.StoredTestResults]

			testOpts.OutputProgress = !parallel

			for _, loc := range locs {
				// XXX Using `dev`'s configuration; ideally we'd run the equivalent of prepare here instead.
				buildEnv := testing.PrepareBuildEnv(ctx, devEnv, ephemeral)

				status := aec.LightBlackF.Apply("RUNNING")
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

					stack, err := stack.Compute(ctx, suts, stack.ProvisionOpts{PortBase: runOpts.BaseServerPort})
					if err != nil {
						return nil, nil, err
					}

					return suts, stack, nil
				})
				if err != nil {
					return fnerrors.UserError(loc, "failed to prepare test: %w", err)
				}

				if parallel || parallelWork {
					parallelTests = append(parallelTests, test)
				} else {
					v, err := compute.Get(ctx, test)
					if err != nil {
						return err
					}

					printResult(stderr, v, false)

					if !v.Value.Bundle.Result.Success {
						return fnerrors.ExitWithCode(fmt.Errorf("test %s failed", v.Value.Package), exitCode)
					}
				}
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

			var failed []string
			if len(parallelTests) > 0 {
				runTests := compute.Collect(tasks.Action("test.all-tests"), parallelTests...)

				results, err := compute.GetValue(testCtx, runTests)
				if err != nil {
					return err
				}

				for _, res := range results {
					printResult(stderr, res, true)
					if !res.Value.Bundle.Result.Success {
						failed = append(failed, string(res.Value.Package))
					}
				}
			}

			if len(failed) > 0 {
				return fnerrors.ExitWithCode(fmt.Errorf("failed tests: [%s]", strings.Join(failed, ",")), exitCode)
			}

			return nil
		}),
	}

	cmd.Flags().Int32Var(&runOpts.BaseServerPort, "port_base", 40000, "Base port to listen on (additional requested ports will be base port + n).")
	cmd.Flags().BoolVar(&testOpts.Debug, "debug", testOpts.Debug, "If true, the testing runtime produces additional information for debugging-purposes.")
	cmd.Flags().BoolVar(&ephemeral, "ephemeral", ephemeral, "If true, don't cleanup any runtime resources created for test (e.g. corresponding Kubernetes namespace).")
	cmd.Flags().BoolVar(&includeServers, "include_servers", includeServers, "If true, also include generated server startup-tests.")
	cmd.Flags().BoolVar(&parallel, "parallel", parallel, "If true, run tests in parallel.")
	cmd.Flags().BoolVar(&parallelWork, "parallel_work", false, "If true, performs all work in parallel except running the actual test (e.g. builds).")
	cmd.Flags().BoolVar(&testing.UseVClusters, "vcluster", testing.UseVClusters, "If true, creates a separate vcluster per test invocation.")

	return cmd
}

func printResult(out io.Writer, res compute.ResultWithTimestamp[testing.StoredTestResults], printResults bool) {
	status := aec.GreenF.Apply("PASSED")
	if !res.Value.Bundle.Result.Success {
		if printResults {
			for _, srv := range res.Value.Bundle.ServerLog {
				printLog(out, srv)
			}
			printLog(out, res.Value.Bundle.TestLog)
		}

		status = aec.RedF.Apply("FAILED")
	}

	cached := ""
	if res.Cached {
		cached = aec.LightBlackF.Apply(" (CACHED)")
	}

	fmt.Fprintf(out, "%s: Test %s%s %s\n", res.Value.Package, status, cached, aec.LightBlackF.Apply(res.Value.ImageRef.ImageRef()))
}

func printLog(out io.Writer, log *testing.Log) {
	for _, line := range bytes.Split(log.Output, []byte("\n")) {
		fmt.Fprintf(out, "%s:%s: %s\n", log.PackageName, log.ContainerName, line)
	}
}
