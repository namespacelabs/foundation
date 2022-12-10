// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"

	containerdlog "github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/binary/genbinary"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/codegen"
	"namespacelabs.dev/foundation/internal/codegen/genpackage"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/filewatcher"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	dockerfileparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/dockerfile"
	goparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/golang"
	nodejsparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/nodejs"
	shellparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/shellscript"
	webparser "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/web"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontendopaque"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/integrations/golang"
	nodebinary "namespacelabs.dev/foundation/internal/integrations/nodejs/binary"
	nodeopaqueintegration "namespacelabs.dev/foundation/internal/integrations/nodejs/opaqueintegration"
	"namespacelabs.dev/foundation/internal/integrations/opaque"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	dockerfileapplier "namespacelabs.dev/foundation/internal/parsing/integration/dockerfile"
	goapplier "namespacelabs.dev/foundation/internal/parsing/integration/golang"
	nodejsapplier "namespacelabs.dev/foundation/internal/parsing/integration/nodejs"
	shellapplier "namespacelabs.dev/foundation/internal/parsing/integration/shellscript"
	webapplier "namespacelabs.dev/foundation/internal/parsing/integration/web"
	"namespacelabs.dev/foundation/internal/planning/deploy"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	artifactregistry "namespacelabs.dev/foundation/internal/providers/gcp/registry"
	k3dp "namespacelabs.dev/foundation/internal/providers/k3d"
	"namespacelabs.dev/foundation/internal/providers/k3s"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/internal/ulimit"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/internal/welcome"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/orchestration"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/actiontracing"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
	"namespacelabs.dev/foundation/universe/aws/iam"
)

var (
	enableErrorTracing   = false
	disableCommandBundle = false
)

func DoMain(name string, registerCommands func(*cobra.Command)) {
	if v := os.Getenv("FN_CPU_PROFILE"); v != "" {
		done := cpuprofile(v)
		defer done()
	}

	SetupViper()

	// These are required for nsboot.
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	rootCtx, style, flushLogs := SetupContext(context.Background())

	// Before moving forward, we check if there's a more up-to-date ns we should fork to.
	if ver, err := version.Current(); err == nil {
		if !nsboot.SpawnedFromBoot() && !version.IsDevelopmentBuild(ver) {
			cached, ns, err := nsboot.CheckUpdate(rootCtx, true, ver.Version)
			if err == nil && cached != nil {
				flushLogs()

				ns.ExecuteAndForwardExitCode(rootCtx, style)
				// Never gets here.
			}
		}
	}

	var cleanupTracer func()
	if tracerEndpoint := viper.GetString("jaeger_endpoint"); tracerEndpoint != "" && viper.GetBool("enable_tracing") {
		rootCtx, cleanupTracer = actiontracing.SetupTracing(rootCtx, tracerEndpoint)
	}

	// Some of our builds can go fairly wide on parallelism, requiring opening
	// hundreds of files, between cache reads, cache writes, etc. This is a best
	// effort attempt at increasing the file limit to a number we can be more
	// comfortable with. 4096 is the result of experimentation.
	ulimit.SetFileLimit(rootCtx, 4096)

	var run *storedrun.Run
	var useTelemetry bool

	var deprecatedToolsInvocation bool

	rootCmd := newRoot(name, func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		tel := fnapi.TelemetryOn(ctx)

		// XXX move id management out of telemetry, it's used for other purposes too.
		if tel.IsFirstRun() && !environment.IsRunningInCI() {
			// First NS run - print a welcome message.
			welcome.PrintWelcome(ctx, true /* firstRun */)
		}

		// Now that "useTelemetry" flag is parsed, we can conditionally enable telemetry.
		if useTelemetry {
			tel.Enable()
		}

		if viper.GetBool("enable_pprof") {
			go ListenPProf(console.Debug(cmd.Context()))
		}

		run = storedrun.New()

		// Used for devhost/environment validation.
		devhost.HasRuntime = runtime.HasRuntime

		parsing.ModuleLoader = cuefrontend.ModuleLoader
		parsing.MakeFrontend = func(pl parsing.EarlyPackageLoader, env *schema.Environment) parsing.Frontend {
			return cuefrontend.NewFrontend(pl, cuefrontendopaque.NewFrontend(env, pl), env)
		}

		filewatcher.SetupFileWatcher()

		binary.BuildGo = golang.GoBuilder
		binary.BuildLLBGen = genbinary.LLBBinary
		binary.BuildAlpine = genbinary.BuildAlpine
		binary.BuildNix = genbinary.NixImageBuilder
		binary.BuildNodejs = nodebinary.NodejsBuilder
		binary.BuildStaticFilesServer = genbinary.StaticFilesServerBuilder

		// Setting up container registry logging, which is unfortunately global.
		logs.Warn = log.New(console.TypedOutput(cmd.Context(), "cr-warn", common.CatOutputTool), "", log.LstdFlags|log.Lmicroseconds)

		parsing.ExtendNodeHook = append(parsing.ExtendNodeHook, func(ctx context.Context, packages pkggraph.PackageLoader, l pkggraph.Location, n *schema.Node) (*parsing.ExtendNodeHookResult, error) {
			// Resolve doesn't require that the package actually exists. It just forces loading the module.
			nodeloc, err := packages.Resolve(ctx, runtime.GrpcHttpTranscodeNode)
			if err != nil {
				return nil, err
			}

			// Check if the namespace version we depend on would have the transcode node.
			ws := nodeloc.Module.Workspace
			if ws.GetFoundation().MinimumApi >= versions.IntroducedGrpcTranscodeNode {
				if n.ExportServicesAsHttp {
					return &parsing.ExtendNodeHookResult{
						Import: []schema.PackageName{runtime.GrpcHttpTranscodeNode},
					}, nil
				}
			}

			return nil, nil
		})

		// Runtime
		tool.RegisterInjection("schema.ComputedNaming", func(ctx context.Context, env cfg.Context, planner runtime.Planner, s *schema.Stack_Entry) (*schema.ComputedNaming, error) {
			n, err := runtime.ComputeNaming(ctx, env.Workspace().ModuleName(), env, planner, s.ServerNaming)
			if n == nil && err == nil {
				return &schema.ComputedNaming{}, nil // these type of injections can't return nil.
			}
			return n, err
		})

		deploy.RegisterDeployOps()

		// Compute cacheables.
		oci.RegisterImageCacheable()

		// Languages.
		golang.Register()
		opaque.Register()
		nodeopaqueintegration.Register()

		// Opaque integrations: parsing
		integrationparsing.IntegrationParser = entity.NewDispatchingEntityParser("kind", []entity.EntityParser{
			&dockerfileparser.Parser{},
			shellparser.NewParser(),
			goparser.NewParser(),
			&nodejsparser.Parser{},
			&webparser.Parser{},
		})
		integrationparsing.BuildParser = entity.NewDispatchingEntityParser("kind", []entity.EntityParser{
			// Same syntax as docker integration so we can reuse the parser.
			&dockerfileparser.Parser{},
			shellparser.NewParser(),
			// Same syntax as go integration so we can reuse the parser.
			goparser.NewParser(),
		})

		// Opaque integrations: applying
		dockerfileapplier.Register()
		goapplier.Register()
		nodejsapplier.Register()
		webapplier.Register()
		shellapplier.Register()

		// Codegen
		genpackage.Register()
		codegen.RegisterGraphHandlers()

		// Providers.
		ecr.Register()
		eks.Register()
		oci.RegisterDomainKeychain("pkg.dev", artifactregistry.DefaultKeychain, oci.Keychain_UseOnWrites)
		iam.RegisterGraphHandlers()
		nscloud.Register()
		k3dp.Register()
		k3s.Register()

		// Runtimes.
		kubernetes.Register()
		kubeops.Register()
		orchestration.RegisterPrepare()

		cfg.ValidateNoConfigTypeCollisions()

		// Telemetry.
		tel.RecordInvocation(ctx, cmd, args)

		if deprecatedToolsInvocation {
			fmt.Fprintf(console.Warnings(ctx), "Flag without effect: --tools_invocation_can_use_buildkit is now the default.\n")
		}

		out := logrus.New()
		out.SetOutput(console.NamedDebug(ctx, "containerd"))
		// Because we can have concurrent builds producing the same output; the
		// local content store implementation will attempt to lock the ref
		// before writing to it. And it will at times fail with
		// codes.Unavailable as it didn't manage to acquire the lock. We need
		// to build deduping for this to go away. NSL-405
		containerdlog.L = logrus.NewEntry(out)
		return nil
	})

	tasks.SetupFlags(rootCmd.PersistentFlags())
	consolesink.SetupFlags(rootCmd.PersistentFlags())
	fnapi.SetupFlags(rootCmd.PersistentFlags())
	nscloud.SetupFlags(rootCmd.PersistentFlags())

	rootCmd.PersistentFlags().BoolVar(&binary.UsePrebuilts, "use_prebuilts", binary.UsePrebuilts,
		"If set to false, binaries are built from source rather than a corresponding prebuilt being used.")
	rootCmd.PersistentFlags().BoolVar(&disableCommandBundle, "disable_command_bundle", disableCommandBundle,
		"If set to true, diagnostics and error information are disabled for the command and the command is filtered from `ns command-history`.")
	rootCmd.PersistentFlags().BoolVar(&console.DebugToConsole, "debug_to_console", console.DebugToConsole,
		"If set to true, we also output debug log messages to the console.")
	rootCmd.PersistentFlags().BoolVar(&compute.CachingEnabled, "caching", compute.CachingEnabled,
		"If set to false, compute caching is disabled.")
	rootCmd.PersistentFlags().BoolVar(&git.AssumeSSHAuth, "git_ssh_auth", !environment.IsRunningInCI(),
		"If set to true, assume that you use SSH authentication with git (this enables us to properly instruct git when downloading private repositories).")

	rootCmd.PersistentFlags().Var(buildkit.ImportCacheVar, "buildkit_import_cache", "Internal, set buildkit import-cache.")
	rootCmd.PersistentFlags().Var(buildkit.ExportCacheVar, "buildkit_export_cache", "Internal, set buildkit export-cache.")
	rootCmd.PersistentFlags().StringVar(&buildkit.BuildkitSecrets, "buildkit_secrets", "", "A list of secrets to pass in to buildkit.")
	rootCmd.PersistentFlags().BoolVar(&buildkit.ForwardKeychain, "buildkit_forward_keychain", buildkit.ForwardKeychain, "If set to true, proxy buildkit auth through namespace orchestration.")
	rootCmd.PersistentFlags().BoolVar(&compute.VerifyCaching, "verify_compute_caching", compute.VerifyCaching,
		"Internal, do not use cached contents of compute graph, verify that the cached content matches instead.")
	rootCmd.PersistentFlags().StringVar(&llbutil.GitCredentialsBuildkitSecret, "golang_buildkit_git_credentials_secret", "",
		"If set, go invocations in buildkit get the specified secret mounted as ~/.git-credentials")
	rootCmd.PersistentFlags().BoolVar(&deploy.AlsoDeployIngress, "also_compute_ingress", deploy.AlsoDeployIngress,
		"[development] Set to false, to skip ingress computation.")
	rootCmd.PersistentFlags().BoolVar(&useTelemetry, "send_usage_data", true,
		"If set to false, ns does not upload any usage data.")
	rootCmd.PersistentFlags().BoolVar(&buildkit.SkipExpectedMaxWorkspaceSizeCheck, "skip_buildkit_workspace_size_check", buildkit.SkipExpectedMaxWorkspaceSizeCheck,
		"If set to true, skips our enforcement of the maximum workspace size we're willing to push to buildkit.")
	rootCmd.PersistentFlags().BoolVar(&k3d.IgnoreZfsCheck, "ignore_zfs_check", k3d.IgnoreZfsCheck,
		"If set to true, ignores checking whether the base system is ZFS based.")
	rootCmd.PersistentFlags().BoolVar(&enableErrorTracing, "error_tracing", enableErrorTracing,
		"If set to true, prints a trace of foundation errors leading to the root cause with source info.")
	rootCmd.PersistentFlags().BoolVar(&deploy.RunCodegen, "run_codegen", deploy.RunCodegen, "If set to false, skip codegen.")
	rootCmd.PersistentFlags().BoolVar(&deploy.PushPrebuiltsToRegistry, "deploy_push_prebuilts_to_registry", deploy.PushPrebuiltsToRegistry,
		"If set to true, prebuilts are uploaded to the target registry.")
	rootCmd.PersistentFlags().BoolVar(&oci.ConvertImagesToEstargz, "oci_convert_images_to_estargz", oci.ConvertImagesToEstargz,
		"If set to true, images are converted to estargz before being uploaded to a registry.")
	rootCmd.PersistentFlags().BoolVar(&tool.InvocationDebug, "invocation_debug", tool.InvocationDebug,
		"If set to true, pass --debug to invocations.")
	rootCmd.PersistentFlags().BoolVar(&kubernetes.UseNodePlatformsForProduction, "kubernetes_use_node_platforms_in_production_builds",
		kubernetes.UseNodePlatformsForProduction, "If set to true, queries the target node platforms to determine what platforms to build for.")
	rootCmd.PersistentFlags().StringSliceVar(&kubernetes.ProductionPlatforms, "production_platforms", kubernetes.ProductionPlatforms,
		"The set of platforms that we build production images for.")
	rootCmd.PersistentFlags().BoolVar(&fnapi.NamingForceStored, "fnapi_naming_force_stored",
		fnapi.NamingForceStored, "If set to true, if there's a stored certificate, use it without checking the server.")
	rootCmd.PersistentFlags().BoolVar(&filewatcher.FileWatcherUsePolling, "filewatcher_use_polling",
		filewatcher.FileWatcherUsePolling, "If set to true, uses polling to observe file system events.")
	rootCmd.PersistentFlags().BoolVar(&kubernetes.DeployAsPodsInTests, "kubernetes_deploy_as_pods_in_tests", kubernetes.DeployAsPodsInTests,
		"If true, servers in tests are deployed as pods instead of deployments.")
	rootCmd.PersistentFlags().BoolVar(&compute.ExplainIndentValues, "compute_explain_indent_values", compute.ExplainIndentValues,
		"If true, values output by --explain are indented.")
	rootCmd.PersistentFlags().BoolVar(&deprecatedToolsInvocation, "tools_invocation_can_use_buildkit", false,
		"If set to true, tool invocations will use buildkit whenever possible.")
	rootCmd.PersistentFlags().BoolVar(&testing.UseNamespaceCloud, "testing_use_namespace_cloud", testing.UseNamespaceCloud,
		"If set to true, allocate cluster for tests on demand.")
	rootCmd.PersistentFlags().BoolVar(&testing.UseNamespaceBuildCluster, "testing_use_namespace_cloud_build", testing.UseNamespaceBuildCluster,
		"If set to true, allocate a build cluster for tests.")
	rootCmd.PersistentFlags().BoolVar(&runtime.WorkInProgressUseShortAlias, "runtime_wip_use_short_alias", runtime.WorkInProgressUseShortAlias,
		"If set to true, uses the new ingress name allocator.")
	rootCmd.PersistentFlags().BoolVar(&orchestration.UseOrchestrator, "use_orchestrator", orchestration.UseOrchestrator,
		"If set to true, enables the new incluster deployment orchestrator.")
	rootCmd.PersistentFlags().BoolVar(&orchestration.RenderOrchestratorDeployment, "render_orchestrator_deployment", orchestration.RenderOrchestratorDeployment,
		"If set to true, we print a render wait block while deploying the orchestrator itself.")
	rootCmd.PersistentFlags().BoolVar(&orchestration.UseHeadOrchestrator, "use_head_orchestrator", orchestration.UseHeadOrchestrator,
		"If set to false, we rebuild the orchestrator instead of using the pinned version. For internal testing only.")
	rootCmd.PersistentFlags().BoolVar(&orchestration.SkipVersionCache, "update_orchestrator", orchestration.SkipVersionCache,
		"If set to true, we ignore the orchestrator version cache and force-update to the latest version. For internal testing only.")
	rootCmd.PersistentFlags().BoolVar(&simplelog.AlsoReportStartEvents, "also_report_start_events", simplelog.AlsoReportStartEvents,
		"If set to true, we log a start event for each action, if --log_actions is also set.")

	storedrun.SetupFlags(rootCmd.PersistentFlags())

	knobs.SetupFlags(rootCmd.PersistentFlags())

	// We have too many flags, hide some of them from --help so users can focus on what's important.
	for _, noisy := range []string{
		"buildkit_import_cache",
		"buildkit_export_cache",
		"buildkit_secrets",
		"debug_to_console",
		"disable_command_bundle",
		"filewatcher_use_polling",
		"verify_compute_caching",
		"also_compute_ingress",
		"golang_buildkit_git_credentials_secret",
		"send_usage_data",
		"skip_buildkit_workspace_size_check",
		"ignore_zfs_check",
		"run_tools_on_kubernetes",
		"run_codegen",
		"invocation_debug",
		"kubernetes_use_node_platforms_in_production_builds",
		"production_platforms",
		"fnapi_naming_force_stored",
		"kubernetes_deploy_as_pods_in_tests",
		"compute_explain_indent_values",
		"nodejs_use_native_node",
		"lowlevel_tools_protocol_version",
		"tools_invocation_can_use_buildkit",
		"deploy_push_prebuilts_to_registry",
		"oci_convert_images_to_estargz",
		"runtime_wip_use_short_alias",
		"use_orchestrator",
		"render_orchestrator_deployment",
		"also_report_start_events",
		"buildkit_forward_keychain",
		"use_head_orchestrator",
		"update_orchestrator",
		// Hidden for M0
		"testing_use_namespace_cloud",
		"testing_use_namespace_cloud_build",
		"use_prebuilts",
	} {
		_ = rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	registerCommands(rootCmd)

	debugLog := console.Debug(rootCtx)
	cmdCtx := tasks.ContextWithThrottler(rootCtx, debugLog, tasks.LoadThrottlerConfig(rootCtx, debugLog))
	err := rootCmd.ExecuteContext(cmdCtx)

	if run != nil {
		runErr := run.Output(cmdCtx, err) // If requested, store the run results.
		if err == nil {
			// Make sure that failing to output fails the execution.
			err = runErr
		}
	}

	if flushLogs != nil {
		flushLogs()
	}

	if cleanupTracer != nil {
		cleanupTracer()
	}

	// Ensures deferred routines after invoked gracefully before os.Exit.
	defer handleExit(rootCtx)

	if err != nil && !errors.Is(err, context.Canceled) {
		exitCode := handleExitError(style, err)

		if tel := fnapi.TelemetryOn(rootCtx); tel != nil {
			// Record errors only after the user sees them to hide potential latency implications.
			// We pass the original ctx without sink since logs have already been flushed.
			tel.RecordError(rootCtx, err)
		}

		// Ensures graceful invocation of deferred routines in the block above before we exit.
		panic(exitWithCode{exitCode})
	}
}

type exitWithCode struct{ Code int }

// exit code handler
func handleExit(ctx context.Context) {
	if e := recover(); e != nil {
		if exit, ok := e.(exitWithCode); ok {
			os.Exit(exit.Code)
		}
		panic(e) // not an Exit, bubble up
	}
}

func handleExitError(style colors.Style, err error) int {
	if exitError, ok := err.(fnerrors.ExitError); ok {
		// If we are exiting, because a sub-process failed, don't bother outputting
		// an error again, just forward the appropriate exit code.
		return exitError.ExitCode()
	} else {
		// Only print errors after calling flushLogs above, so the console driver
		// is no longer erasing lines.
		format.Format(os.Stderr, err, format.WithStyle(style), format.WithTracing(enableErrorTracing), format.WithActionTrace(true))
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

		Example: `  ns prepare local  Prepares the local workspace for development or production.
  ns test           Run all functional end-to-end tests in the current workspace.
  ns dev            Starts a development session, continuously building and deploying servers.`,

		// adds the welcome message to `ns`, `ns help` and `ns --help`
		Long: welcome.WelcomeMessage(false /* firstRun*/),
	}
}

func SetupViper() {
	ensureFnConfig()

	viper.SetEnvPrefix("ns")
	viper.SetConfigName("config")
	viper.SetConfigType("json")

	if cfg, err := dirs.Config(); err == nil {
		viper.AddConfigPath(cfg)
	}

	viper.SetDefault("jaeger_endpoint", "")
	_ = viper.BindEnv("jaeger_endpoint")

	viper.SetDefault("console_no_colors", false)
	_ = viper.BindEnv("console_no_colors")

	viper.SetDefault("enable_tracing", false)
	_ = viper.BindEnv("enable_tracing")

	viper.SetDefault("telemetry", true)

	viper.SetDefault("enable_autoupdate", true)
	_ = viper.BindEnv("enable_autoupdate")

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

func ensureFnConfig() {
	if fnDir, err := dirs.Config(); err == nil {
		p := filepath.Join(fnDir, "config.json")
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

func StandardConsole() (*os.File, bool) {
	isStdoutTerm := termios.IsTerm(os.Stdout.Fd())
	isStderrTerm := termios.IsTerm(os.Stderr.Fd())
	return os.Stderr, isStdoutTerm && isStderrTerm
}

func ConsoleToSink(out *os.File, isTerm bool) (tasks.ActionSink, colors.Style, func()) {
	maxLogLevel := viper.GetInt("console_log_level")
	if isTerm && !viper.GetBool("console_no_colors") {
		consoleSink := consolesink.NewSink(out, isTerm, maxLogLevel)
		cleanup := consoleSink.Start()
		return consoleSink, colors.WithColors, cleanup
	}

	return simplelog.NewSink(out, maxLogLevel), colors.NoColors, nil
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

func SetupContext(ctx context.Context) (context.Context, colors.Style, func()) {
	sink, style, flushLogs := ConsoleToSink(StandardConsole())
	ctx = colors.WithStyle(tasks.WithSink(ctx, sink), style)
	if flushLogs == nil {
		flushLogs = func() {}
	}

	return fnapi.WithTelemetry(ctx), style, flushLogs
}
