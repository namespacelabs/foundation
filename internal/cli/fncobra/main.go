// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"errors"
	"fmt"
	"log"
	httppprof "net/http/pprof"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/gorilla/mux"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/binary/genbinary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/console"
	clrs "namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/ulimit"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/languages/golang"
	nodejs "namespacelabs.dev/foundation/languages/nodejs/integration"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/languages/web"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/providers/aws/iam"
	ecr "namespacelabs.dev/foundation/providers/aws/registry"
	artifactregistry "namespacelabs.dev/foundation/providers/gcp/registry"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	src "namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/codegen"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/actiontracing"
)

var (
	enableErrorTracing = false
)

func DoMain(name string, registerCommands func(*cobra.Command)) {
	if v := os.Getenv("FN_CPU_PROFILE"); v != "" {
		done := cpuprofile(v)
		defer done()
	}

	setupViper()

	if viper.GetBool("enable_pprof") {
		go ListenPProf()
	}

	ctx := context.Background()

	var cleanupTracer func()
	if tracerEndpoint := viper.GetString("jaeger_endpoint"); tracerEndpoint != "" && viper.GetBool("enable_tracing") {
		ctx, cleanupTracer = actiontracing.SetupTracing(ctx, tracerEndpoint)
	}

	out, interactive := consoleFromFile()

	logger, sink, flushLogs := consoleToSink(out, interactive)

	ctxWithSink := tasks.WithSink(logger.WithContext(ctx), sink)

	// Some of our builds can go fairly wide on parallelism, requiring opening
	// hundreds of files, between cache reads, cache writes, etc. This is a best
	// effort attempt at increasing the file limit to a number we can be more
	// comfortable with. 4096 is the result of experimentation.
	ulimit.SetFileLimit(ctxWithSink, 4096)

	tel := fnapi.NewTelemetry()

	var remoteStatusChan chan remoteStatus
	// Checking a version could be used for fingerprinting purposes,
	// and thus we don't do it if the user has opted-out from providing data.
	if tel.IsTelemetryEnabled() {
		remoteStatusChan = make(chan remoteStatus)
		go checkRemoteStatus(logger, remoteStatusChan)
	}

	bundler := tasks.NewActionBundler()
	// Delete stale bundles asynchronously on startup.
	defer func() {
		_ = bundler.RemoveStaleBundles()
	}()

	// Create a new in-memory bundle to track actions. The bundle
	// is flushed at the end of command invocation.
	bundle := bundler.NewInMemoryBundle()

	rootCmd := newRoot(name, func(cmd *cobra.Command, args []string) error {
		err := bundle.WriteInvocationInfo(cmd.Context(), cmd, args)
		if err != nil {
			return err
		}

		tasks.ActionStorer = tasks.NewStorer(cmd.Context(), bundler, bundle, tasks.StorerWithFlushLogs(flushLogs))

		if err := tasks.SetupThrottler(console.Debug(cmd.Context())); err != nil {
			return err
		}

		// Used for devhost/environment validation.
		devhost.HasRuntime = runtime.HasRuntime

		workspace.ModuleLoader = cuefrontend.ModuleLoader
		workspace.MakeFrontend = cuefrontend.NewFrontend

		binary.BuildGo = func(loc workspace.Location, goPackage, binName string, unsafeCacheable bool) (build.Spec, error) {
			gobin, err := golang.FromLocation(loc, goPackage)
			if err != nil {
				return nil, fnerrors.Wrap(loc, err)
			}
			gobin.BinaryName = binName
			gobin.UnsafeCacheable = unsafeCacheable
			return gobin, nil
		}
		binary.BuildWeb = func(loc workspace.Location) build.Spec {
			return web.StaticBuild{Location: loc}
		}
		binary.BuildLLBGen = genbinary.LLBBinary
		binary.BuildNix = genbinary.NixImage

		// Setting up container registry logging, which is unfortunately global.
		logs.Warn = log.New(console.TypedOutput(cmd.Context(), "cr-warn", common.CatOutputTool), "", log.LstdFlags|log.Lmicroseconds)

		workspace.ExtendNodeHook = append(workspace.ExtendNodeHook, func(ctx context.Context, packages workspace.Packages, l workspace.Location, n *schema.Node) (*workspace.ExtendNodeHookResult, error) {
			// Resolve doesn't require that the package actually exists. It just forces loading the module.
			nodeloc, err := packages.Resolve(ctx, runtime.GrpcHttpTranscodeNode)
			if err != nil {
				return nil, err
			}

			// Check if the foundation version we depend on would have the transcode node.
			ws := nodeloc.Module.Workspace
			if ws.GetFoundation().MinimumApi >= versions.IntroducedGrpcTranscodeNode {
				if n.ExportServicesAsHttp {
					return &workspace.ExtendNodeHookResult{
						Import: []schema.PackageName{runtime.GrpcHttpTranscodeNode},
					}, nil
				}
			}

			return nil, nil
		})

		// Runtime
		tool.RegisterInjection("schema.ComputedNaming", func(ctx context.Context, env *schema.Environment, s *schema.Stack_Entry) (*schema.ComputedNaming, error) {
			return runtime.ComputeNaming(env, s.ServerNaming)
		})

		// Compute cacheables.
		compute.RegisterProtoCacheable()
		compute.RegisterBytesCacheable()
		fscache.RegisterFSCacheable()
		oci.RegisterImageCacheable()

		// Languages.
		golang.Register()
		web.Register()
		nodejs.Register()
		opaque.Register()

		// Codegen
		codegen.Register()
		src.RegisterGraphHandlers()

		// Providers.
		ecr.Register()
		eks.Register()
		artifactregistry.Register()
		oci.RegisterDomainKeychain("pkg.dev", artifactregistry.DefaultKeychain, oci.Keychain_UseOnWrites)
		oci.RegisterDomainKeychain("amazonaws.com", ecr.DefaultKeychain, oci.Keychain_UseAlways)
		iam.RegisterGraphHandlers()

		// Runtimes.
		kubernetes.Register()
		kubeops.Register()

		// Telemetry.
		tel.RecordInvocation(ctxWithSink, cmd, args)
		return nil
	})

	rootCmd.PersistentFlags().BoolVar(&binary.UsePrebuilts, "use_prebuilts", binary.UsePrebuilts,
		"If set to false, binaries are built from source rather than a corresponding prebuilt being used.")
	rootCmd.PersistentFlags().BoolVar(&consolesink.LogActions, "log_actions", consolesink.LogActions,
		"If set to true, each completed action is also output as a log message.")
	rootCmd.PersistentFlags().BoolVar(&console.DebugToConsole, "debug_to_console", console.DebugToConsole,
		"If set to true, we also output debug log messages to the console.")
	rootCmd.PersistentFlags().BoolVar(&compute.CachingEnabled, "caching", compute.CachingEnabled,
		"If set to false, compute caching is disabled.")
	rootCmd.PersistentFlags().BoolVar(&git.AssumeSSHAuth, "git_ssh_auth", git.AssumeSSHAuth,
		"If set to true, assume that you use SSH authentication with git (this enables us to properly instruct git when downloading private repositories).")

	rootCmd.PersistentFlags().Var(buildkit.ImportCacheVar, "buildkit_import_cache", "Internal, set buildkit import-cache.")
	rootCmd.PersistentFlags().Var(buildkit.ExportCacheVar, "buildkit_export_cache", "Internal, set buildkit export-cache.")
	rootCmd.PersistentFlags().StringVar(&buildkit.BuildkitSecrets, "buildkit_secrets", "", "A list of secrets to pass in to buildkit.")
	rootCmd.PersistentFlags().BoolVar(&compute.VerifyCaching, "verify_compute_caching", compute.VerifyCaching,
		"Internal, do not use cached contents of compute graph, verify that the cached content matches instead.")
	rootCmd.PersistentFlags().BoolVar(&golang.UseBuildKitForBuilding, "golang_use_buildkit", golang.UseBuildKitForBuilding,
		"If set to true, buildkit is used for building, instead of a ko-style builder.")
	rootCmd.PersistentFlags().StringVar(&llbutil.GitCredentialsBuildkitSecret, "golang_buildkit_git_credentials_secret", "",
		"If set, go invocations in buildkit get the specified secret mounted as ~/.git-credentials")
	rootCmd.PersistentFlags().BoolVar(&deploy.AlsoDeployIngress, "also_compute_ingress", deploy.AlsoDeployIngress,
		"[development] Set to false, to skip ingress computation.")
	rootCmd.PersistentFlags().BoolVar(&tel.UseTelemetry, "send_usage_data", tel.UseTelemetry,
		"If set to false, fn does not upload any usage data.")
	rootCmd.PersistentFlags().BoolVar(&buildkit.SkipExpectedMaxWorkspaceSizeCheck, "skip_buildkit_workspace_size_check", buildkit.SkipExpectedMaxWorkspaceSizeCheck,
		"If set to true, skips our enforcement of the maximum workspace size we're willing to push to buildkit.")
	rootCmd.PersistentFlags().BoolVar(&k3d.IgnoreZfsCheck, "ignore_zfs_check", k3d.IgnoreZfsCheck,
		"If set to true, ignores checking whether the base system is ZFS based.")
	rootCmd.PersistentFlags().BoolVar(&enableErrorTracing, "error_tracing", enableErrorTracing,
		"If set to true, prints a trace of foundation errors leading to the root cause with source info.")
	rootCmd.PersistentFlags().BoolVar(&tools.UseKubernetesRuntime, "run_tools_on_kubernetes", tools.UseKubernetesRuntime,
		"If set to true, runs tools in Kubernetes, instead of Docker.")
	rootCmd.PersistentFlags().BoolVar(&deploy.RunCodegen, "run_codegen", deploy.RunCodegen, "If set to false, skip codegen.")
	rootCmd.PersistentFlags().BoolVar(&tool.InvocationDebug, "invocation_debug", tool.InvocationDebug,
		"If set to true, pass --debug to invocations.")
	rootCmd.PersistentFlags().BoolVar(&kubernetes.UseNodePlatformsForProduction, "kubernetes_use_node_platforms_in_production_builds",
		kubernetes.UseNodePlatformsForProduction, "If set to true, queries the target node platforms to determine what platforms to build for.")
	rootCmd.PersistentFlags().StringSliceVar(&kubernetes.ProductionPlatforms, "production_platforms", kubernetes.ProductionPlatforms,
		"The set of platforms that we build production images for.")

	// We have too many flags, hide some of them from --help so users can focus on what's important.
	for _, noisy := range []string{
		"buildkit_import_cache",
		"buildkit_export_cache",
		"buildkit_secrets",
		"verify_compute_caching",
		"also_compute_ingress",
		"golang_use_buildkit",
		"golang_buildkit_git_credentials_secret",
		"send_usage_data",
		"skip_buildkit_workspace_size_check",
		"ignore_zfs_check",
		"run_tools_on_kubernetes",
		"run_codegen",
		"invocation_debug",
		"kubernetes_use_node_platforms_in_production_builds",
		"production_platforms",
	} {
		_ = rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	registerCommands(rootCmd)

	err := rootCmd.ExecuteContext(ctxWithSink)

	if flushLogs != nil {
		flushLogs()
	}

	if cleanupTracer != nil {
		cleanupTracer()
	}

	// Check if this is a version requirement error, if yes, skip the regular version checker.
	if _, ok := err.(*fnerrors.VersionError); !ok && remoteStatusChan != nil {
		// Printing the new version message if any.
		select {
		case status, ok := <-remoteStatusChan:
			if ok {
				clr := clrs.Green
				if !interactive {
					clr = func(str string) string { return str }
				}

				var messages []string

				if status.NewVersion {
					messages = append(messages, fmt.Sprintf("New Foundation release %s is available.\nDownload: %s", status.Version, downloadUrl(status.Version)))
				}

				if status.Message != "" {
					if len(messages) > 0 {
						messages = append(messages, "")
					}
					messages = append(messages, status.Message)
				}

				if len(messages) > 0 {
					fmt.Fprintln(os.Stdout, clr(strings.Join(messages, "\n")))
				}
			}
		default:
		}
	}

	// Ensures deferred routines after invoked gracefully before os.Exit.
	defer handleExit()
	defer func() {
		// Capture useful information about the environment helpful for diagnostics in the bundle.
		_ = writeExitInfo(ctxWithSink, bundle)

		// Commit the bundle to the filesystem.
		_ = bundler.Flush(ctxWithSink, bundle)
	}()

	if err != nil && !errors.Is(err, context.Canceled) {
		exitCode := handleExitError(interactive, err)
		// Record errors only after the user sees them to hide potential latency implications.
		// We pass the original ctx without sink since logs have already been flushed.
		tel.RecordError(ctx, err)
		_ = bundle.WriteError(ctx, err)

		// Ensures graceful invocation of deferred routines in the block above before we exit.
		panic(exitWithCode{exitCode})
	}
}

type exitWithCode struct{ Code int }

// exit code handler
func handleExit() {
	if e := recover(); e != nil {
		if exit, ok := e.(exitWithCode); ok {
			os.Exit(exit.Code)
		}
		panic(e) // not an Exit, bubble up
	}
}

func writeExitInfo(ctx context.Context, bundle *tasks.Bundle) error {
	err := bundle.WriteMemStats(ctx)
	if err != nil {
		return err
	}
	client, err := docker.NewClient()
	if err != nil {
		return err
	}
	dockerInfo, err := client.Info(ctx)
	if err != nil {
		return err
	}
	return bundle.WriteDockerInfo(ctx, &dockerInfo)
}

func handleExitError(colors bool, err error) int {
	if exitError, ok := err.(fnerrors.ExitError); ok {
		// If we are exiting, because a sub-process failed, don't bother outputting
		// an error again, just forward the appropriate exit code.
		return exitError.ExitCode()
	} else if versionError, ok := err.(*fnerrors.VersionError); ok {
		fnerrors.Format(os.Stderr, versionError, fnerrors.WithColors(true))

		if version, err := version.Version(); err == nil {
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if status, err := FetchLatestRemoteStatus(ctxWithTimeout, versionCheckEndpoint, version.GitCommit); err == nil && status.Version != "" {
				fmt.Fprintln(os.Stderr, indent.String(
					wordwrap.String(
						fmt.Sprintf("\nThe latest version of Foundation is %s, available at %s\n", clrs.Bold(status.Version), downloadUrl(status.Version)),
						80),
					2))
			}
		}

		return 2
	} else {
		// Only print errors after calling flushLogs above, so the console driver
		// is no longer erasing lines.
		fnerrors.Format(os.Stderr, err, fnerrors.WithColors(true), fnerrors.WithTracing(enableErrorTracing))
		return 1
	}
}

func newRoot(name string, preRunE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	return &cobra.Command{
		Use: name,

		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,

		PersistentPreRunE: preRunE,

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.SetOut(console.Stderr(cmd.Context()))
				cmd.HelpFunc()(cmd, args)
				return nil
			}

			return fmt.Errorf("%s: '%s' is not a %s command.\nSee '%s --help'", name, args[0], name, name)
		},
	}
}

func setupViper() {
	ensureFnConfig()

	viper.SetEnvPrefix("fn")
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	if cfg, err := dirs.Config(); err == nil {
		viper.AddConfigPath(cfg)
	}

	viper.SetDefault("log_level", "info")
	_ = viper.BindEnv("log_level")

	viper.SetDefault("jaeger_endpoint", "")
	_ = viper.BindEnv("jaeger_endpoint")

	viper.SetDefault("console_output", "text")
	_ = viper.BindEnv("console_output")

	viper.SetDefault("console_no_colors", false)
	_ = viper.BindEnv("console_no_colors")

	viper.SetDefault("enable_tracing", false)
	_ = viper.BindEnv("enable_tracing")

	viper.SetDefault("enable_telemetry", true)

	viper.SetDefault("console_log_level", 0)
	_ = viper.BindEnv("console_log_level")

	viper.SetDefault("enable_pprof", false)
	_ = viper.BindEnv("enable_pprof")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatal(err)
		}
	}
}

func downloadUrl(version string) string {
	return fmt.Sprintf("https://github.com/namespacelabs/foundation/releases/tag/%s", version)
}

func ensureFnConfig() {
	if fnDir, err := dirs.Config(); err == nil {
		p := fmt.Sprintf("%s/config.json", fnDir)
		if _, err := os.Stat(p); err == nil {
			// Already exists.
			return
		}

		if err := os.MkdirAll(fnDir, 0755); err == nil {
			if f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644); err == nil {
				// Ignore errors.
				fmt.Fprintf(f, "{}\n")
				f.Close()
			}
		}
	}
}

func consoleFromFile() (*os.File, bool) {
	out := os.Stderr
	return out, termios.IsTerm(out.Fd())
}

func outputType() logoutput.OutputType {
	if strings.ToLower(viper.GetString("console_output")) == "json" {
		return logoutput.OutputJSON
	}
	return logoutput.OutputText
}

func consoleToSink(out *os.File, interactive bool) (*zerolog.Logger, tasks.ActionSink, func()) {
	logout := logoutput.OutputTo{Writer: out, WithColors: interactive, OutputType: outputType()}

	maxLogLevel := viper.GetInt("console_log_level")
	if (interactive || environment.IsRunningInCI()) && !viper.GetBool("console_no_colors") {
		consoleSink := consolesink.NewSink(out, interactive, maxLogLevel)
		cleanup := consoleSink.Start()
		logout.Writer = console.ConsoleOutput(consoleSink, common.KnownStderr)

		return logout.ZeroLogger(), consoleSink, cleanup
	}

	logger := logout.ZeroLogger()

	return logger, tasks.NewJsonLoggerSink(logger, maxLogLevel), nil
}

func cpuprofile(cpuprofile string) func() {
	f, err := os.Create(cpuprofile)
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	return func() {
		pprof.StopCPUProfile()
		f.Close()
	}
}

// Checks for updates and messages from Foundation developers.
// Does nothing if a check for remote status failed
func checkRemoteStatus(logger *zerolog.Logger, channel chan remoteStatus) {
	defer close(channel)

	ver, err := version.Version()
	if err != nil {
		logger.Debug().Err(err).Msg("failed to obtain version information")
		return
	}

	if ver.BuildTime == nil || ver.Version == version.DevelopmentBuildVersion {
		return // Nothing to check.
	}

	logger.Debug().Stringer("binary_build_time", ver.BuildTime).Msg("version check")

	status, err := FetchLatestRemoteStatus(context.Background(), versionCheckEndpoint, ver.GitCommit)
	if err != nil {
		logger.Debug().Err(err).Msg("version check failed")
	} else {
		logger.Debug().Stringer("latest_release_version", status.BuildTime).Msg("version check")

		if semver.Compare(status.Version, ver.Version) > 0 {
			status.NewVersion = true
		}

		channel <- *status
	}
}

func RegisterPprof(r *mux.Router) {
	r.PathPrefix("/debug/pprof/").HandlerFunc(httppprof.Index)
	r.PathPrefix("/debug/pprof/cmdline").HandlerFunc(httppprof.Cmdline)
	r.PathPrefix("/debug/pprof/profile").HandlerFunc(httppprof.Profile)
	r.PathPrefix("/debug/pprof/symbol").HandlerFunc(httppprof.Symbol)
	r.PathPrefix("/debug/pprof/trace").HandlerFunc(httppprof.Trace)
	r.PathPrefix("/debug/pprof/goroutine").HandlerFunc(httppprof.Index)
}
