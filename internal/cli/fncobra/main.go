// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/muesli/reflow/indent"
	"github.com/muesli/reflow/wordwrap"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/binary/genbinary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/filewatcher"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/nodejs"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/testing"
	"namespacelabs.dev/foundation/internal/ulimit"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/languages/golang"
	nodeintegration "namespacelabs.dev/foundation/languages/nodejs/integration"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/languages/web"
	"namespacelabs.dev/foundation/providers/aws/ecr"
	"namespacelabs.dev/foundation/providers/aws/eks"
	"namespacelabs.dev/foundation/providers/aws/iam"
	artifactregistry "namespacelabs.dev/foundation/providers/gcp/registry"
	"namespacelabs.dev/foundation/providers/nscloud"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeops"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/codegen"
	"namespacelabs.dev/foundation/workspace/tasks"
	"namespacelabs.dev/foundation/workspace/tasks/actiontracing"
	"namespacelabs.dev/foundation/workspace/tasks/simplelog"
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

	ctx := context.Background()

	var cleanupTracer func()
	if tracerEndpoint := viper.GetString("jaeger_endpoint"); tracerEndpoint != "" && viper.GetBool("enable_tracing") {
		ctx, cleanupTracer = actiontracing.SetupTracing(ctx, tracerEndpoint)
	}

	tel := fnapi.NewTelemetry()

	sink, style, flushLogs := ConsoleToSink(StandardConsole())
	ctxWithSink := fnapi.WithTelemetry(colors.WithStyle(tasks.WithSink(ctx, sink), style), tel)

	debugSink := console.Debug(ctx)

	// Some of our builds can go fairly wide on parallelism, requiring opening
	// hundreds of files, between cache reads, cache writes, etc. This is a best
	// effort attempt at increasing the file limit to a number we can be more
	// comfortable with. 4096 is the result of experimentation.
	ulimit.SetFileLimit(ctxWithSink, 4096)

	var remoteStatusChan chan remoteStatus

	cmdBundle := NewCommandBundle(disableCommandBundle)
	// Remove stale commands asynchronously on startup.
	defer func() {
		_ = cmdBundle.RemoveStaleCommands()
	}()

	var run *storedrun.Run
	var useTelemetry bool

	rootCmd := newRoot(name, func(cmd *cobra.Command, args []string) error {
		// Now that "useTelemetry" flag is parsed, we can conditionally enable telemetry.
		if useTelemetry {
			tel.Enable()
		}

		if viper.GetBool("enable_pprof") {
			go ListenPProf(console.Debug(cmd.Context()))
		}

		if err := cmdBundle.RegisterCommand(cmd, args); err != nil {
			return err
		}

		tasks.ActionStorer = cmdBundle.CreateActionStorer(cmd.Context(), flushLogs)

		run = storedrun.New()

		// Used for devhost/environment validation.
		devhost.HasRuntime = runtime.HasRuntime

		workspace.ModuleLoader = cuefrontend.ModuleLoader
		workspace.MakeFrontend = cuefrontend.NewFrontend

		filewatcher.SetupFileWatcher()

		// Checking a version could be used for fingerprinting purposes,
		// and thus we don't do it if the user has opted-out from providing data.
		if tel.IsTelemetryEnabled() {
			remoteStatusChan = make(chan remoteStatus)
			// Remote status check may linger until after we flush the main sink.
			// So here we just suppress any messages from the background version check.
			ctxWithNullSink := tasks.WithSink(ctx, tasks.NullSink())
			// NB: Requires workspace.ModuleLoader.
			go checkRemoteStatus(ctxWithNullSink, remoteStatusChan)
		}

		binary.BuildGo = func(loc workspace.Location, plan *schema.ImageBuildPlan_GoBuild, unsafeCacheable bool) (build.Spec, error) {
			gobin, err := golang.FromLocation(loc, plan.RelPath)
			if err != nil {
				return nil, fnerrors.Wrap(loc, err)
			}
			gobin.BinaryOnly = plan.BinaryOnly
			gobin.BinaryName = plan.BinaryName
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

			// Check if the namespace version we depend on would have the transcode node.
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
		tool.RegisterInjection("schema.ComputedNaming", func(ctx context.Context, env ops.Environment, s *schema.Stack_Entry) (*schema.ComputedNaming, error) {
			return runtime.ComputeNaming(ctx, env.Workspace().ModuleName, env.Proto(), s.ServerNaming)
		})

		// Compute cacheables.
		compute.RegisterProtoCacheable()
		compute.RegisterBytesCacheable()
		fscache.RegisterFSCacheable()
		oci.RegisterImageCacheable()

		// Languages.
		golang.Register()
		web.Register()
		nodeintegration.Register()
		opaque.Register()

		// Codegen
		codegen.Register()
		source.RegisterGraphHandlers()

		// Providers.
		ecr.Register()
		eks.Register()
		oci.RegisterDomainKeychain("pkg.dev", artifactregistry.DefaultKeychain, oci.Keychain_UseOnWrites)
		iam.RegisterGraphHandlers()
		nscloud.RegisterRegistry()
		nscloud.RegisterClusterProvider()

		// Runtimes.
		kubernetes.Register()
		kubeops.Register()

		// Telemetry.
		tel.RecordInvocation(ctxWithSink, cmd, args)
		return nil
	})

	tasks.SetupFlags(rootCmd.PersistentFlags())
	fnapi.SetupFlags(rootCmd.PersistentFlags())

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
	rootCmd.PersistentFlags().BoolVar(&compute.VerifyCaching, "verify_compute_caching", compute.VerifyCaching,
		"Internal, do not use cached contents of compute graph, verify that the cached content matches instead.")
	rootCmd.PersistentFlags().BoolVar(&golang.UseBuildKitForBuilding, "golang_use_buildkit", golang.UseBuildKitForBuilding,
		"If set to true, buildkit is used for building, instead of a ko-style builder.")
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
	rootCmd.PersistentFlags().BoolVar(&tools.UseKubernetesRuntime, "run_tools_on_kubernetes", tools.UseKubernetesRuntime,
		"If set to true, runs tools in Kubernetes, instead of Docker.")
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
	rootCmd.PersistentFlags().BoolVar(&nodejs.UseNativeNode, "nodejs_use_native_node", nodejs.UseNativeNode, "If true, invokes a locally installed node.")
	rootCmd.PersistentFlags().IntVar(&tools.LowLevelToolsProtocolVersion, "lowlevel_tools_protocol_version", tools.LowLevelToolsProtocolVersion,
		"The protocol version to use with invocation tools.")
	rootCmd.PersistentFlags().BoolVar(&tools.InvocationCanUseBuildkit, "tools_invocation_can_use_buildkit", tools.InvocationCanUseBuildkit,
		"If set to true, tool invocations will use buildkit whenever possible.")
	rootCmd.PersistentFlags().BoolVar(&testing.UseNamespaceCloud, "testing_use_namespace_cloud", testing.UseNamespaceCloud,
		"If set to true, allocate cluster for tests on demand.")
	rootCmd.PersistentFlags().BoolVar(&runtime.WorkInProgressUseShortAlias, "runtime_wip_use_short_alias", runtime.WorkInProgressUseShortAlias,
		"If set to true, uses the new ingress name allocator.")

	cmdBundle.SetupFlags(rootCmd.PersistentFlags())
	storedrun.SetupFlags(rootCmd.PersistentFlags())

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
		"fnapi_naming_force_stored",
		"kubernetes_deploy_as_pods_in_tests",
		"compute_explain_indent_values",
		"nodejs_use_native_node",
		"lowlevel_tools_protocol_version",
		"tools_invocation_can_use_buildkit",
		"deploy_push_prebuilts_to_registry",
		"oci_convert_images_to_estargz",
		"runtime_wip_use_short_alias",
	} {
		_ = rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	registerCommands(rootCmd)

	cmdCtx := tasks.ContextWithThrottler(ctxWithSink, console.Debug(ctx), tasks.LoadThrottlerConfig(ctx, console.Debug(ctx)))
	err := rootCmd.ExecuteContext(cmdCtx)

	serializedErrToBundle := false

	if run != nil {
		if err != nil {
			// This is a write into an in-memory FS and does not incur any I/O overhead.
			if writeErr := cmdBundle.WriteError(ctx, err); writeErr != nil {
				fmt.Fprintf(debugSink, "Failed to serialize the command execution error in the bundle: %v\n", writeErr)
			}
			// We set the bit to ensure we don't try re-serializing the error.
			serializedErrToBundle = true
		}

		actionLogs, logErr := cmdBundle.ActionLogs(ctxWithSink)
		if actionLogs != nil {
			storedrun.Attach(actionLogs)
		}

		if logErr != nil {
			fmt.Fprintf(debugSink, "Failed to write action logs: %v\n", logErr)
		}

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

	// Check if this is a version requirement error, if yes, skip the regular version checker.
	if _, ok := err.(*fnerrors.VersionError); !ok && remoteStatusChan != nil {
		select {
		case status, ok := <-remoteStatusChan:
			if ok && status.NewVersion {
				fmt.Fprintf(os.Stdout, "New Namespace release %s is available.\nDownload: %s",
					status.Version, downloadUrl(status.Version))
			}
		default:
		}
	}

	// Ensures deferred routines after invoked gracefully before os.Exit.
	defer handleExit(ctx)
	defer func() {
		// Capture useful information about the environment helpful for diagnostics in the bundle.
		if flushErr := cmdBundle.FlushWithExitInfo(ctxWithSink); flushErr != nil {
			fmt.Fprintf(debugSink, "Failed to flush the bundle with exit info: %v\n", flushErr)
		}
	}()

	if err != nil && !errors.Is(err, context.Canceled) {
		exitCode := handleExitError(style, err, remoteStatusChan)

		if tel != nil {
			// Record errors only after the user sees them to hide potential latency implications.
			// We pass the original ctx without sink since logs have already been flushed.
			tel.RecordError(ctx, err)
		}

		// Ensure that the error with stack trace is a part of the command bundle if not written already.
		if !serializedErrToBundle {
			if writeErr := cmdBundle.WriteError(ctx, err); writeErr != nil {
				fmt.Fprintf(debugSink, "Failed to serialize the command execution error in the bundle: %v\n", writeErr)
			}
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

func handleExitError(style colors.Style, err error, remoteStatusChan chan remoteStatus) int {
	if exitError, ok := err.(fnerrors.ExitError); ok {
		// If we are exiting, because a sub-process failed, don't bother outputting
		// an error again, just forward the appropriate exit code.
		return exitError.ExitCode()
	} else if versionError, ok := err.(*fnerrors.VersionError); ok {
		fnerrors.Format(os.Stderr, versionError, fnerrors.WithStyle(style))

		select {
		case <-time.After(5 * time.Second):
		case status, ok := <-remoteStatusChan:
			if ok {
				fmt.Fprintln(os.Stderr, indent.String(
					wordwrap.String(
						fmt.Sprintf("\nThe latest version of Namespace is %s, available at %s\n",
							style.Highlight.Apply(status.Version), downloadUrl(status.Version)),
						80),
					2))
			}
		}

		return 2
	} else {
		// Only print errors after calling flushLogs above, so the console driver
		// is no longer erasing lines.
		fnerrors.Format(os.Stderr, err, fnerrors.WithStyle(style), fnerrors.WithTracing(enableErrorTracing))
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

		Example: `  ns create starter Creates a new workspace from a template.
  ns prepare local  Prepares the local workspace for development or production.
  ns test           Run all functional end-to-end tests in the current workspace.
  ns dev            Starts a development session, continuously building and deploying servers.`,
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

	viper.SetDefault("enable_telemetry", true)

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
