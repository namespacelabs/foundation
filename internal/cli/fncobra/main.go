// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncobra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"

	containerdlog "github.com/containerd/containerd/log"
	crlogs "github.com/google/go-containerregistry/pkg/logs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/ulimit"
	"namespacelabs.dev/foundation/internal/clerk"
	fncobraname "namespacelabs.dev/foundation/internal/cli/fncobra/name"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/welcome"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/actiontracing"
	"namespacelabs.dev/foundation/std/tasks/idtypes"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
)

var (
	enableErrorTracing   = false
	disableCommandBundle = false
)

type MainOpts struct {
	Name                 string
	AutoUpdate           bool
	NotifyOnNewVersion   bool
	FormatErr            FormatErrorFunc
	ConsoleInhibitReport bool
	ConsoleRenderer      consolesink.RendererFunc
	RegisterCommands     func(*cobra.Command)
}

func DoMain(opts MainOpts) {
	fncobraname.CmdName = opts.Name

	if info, ok := debug.ReadBuildInfo(); ok {
		if v, err := version.VersionFrom(info); err == nil {
			fnapi.UserAgent = fmt.Sprintf("%s/%s", opts.Name, v.Version)
		}
	}

	style, err := doMain(opts)

	if err != nil && !errors.Is(err, context.Canceled) {
		format := opts.FormatErr
		if format == nil {
			format = DefaultErrorFormatter
		}

		os.Exit(handleExitError(style, err, format))
	}
}

func doMain(opts MainOpts) (colors.Style, error) {
	if v := os.Getenv("FN_CPU_PROFILE"); v != "" {
		done := cpuprofile(v)
		defer done()
	}

	SetupViper()

	// These are required for nsboot.
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	rootCtx, style, flushLogs := setupContext(context.Background(), opts.ConsoleInhibitReport, opts.ConsoleRenderer)

	// Before moving forward, we check if there's a more up-to-date ns we should fork to.
	if opts.AutoUpdate && opts.Name == "ns" { // Applies only to ns, not nsc and docker-credential-helper
		maybeRunLatest(rootCtx, style, flushLogs, opts.Name)
	} else {
		maybeRunLatestFromCache(rootCtx, style, flushLogs, opts.Name)
	}

	var cleanupTracer func()
	if tracerEndpoint := viper.GetString("jaeger_endpoint"); tracerEndpoint != "" && viper.GetBool("enable_tracing") {
		rootCtx, cleanupTracer = actiontracing.SetupTracing(rootCtx, tracerEndpoint)
	}

	// Some of our builds can go fairly wide on parallelism, requiring opening
	// hundreds of files, between cache reads, cache writes, etc. This is a best
	// effort attempt at increasing the file limit to a number we can be more
	// comfortable with. 4096 is the result of experimentation.
	if err := ulimit.SetFileLimit(4096); err != nil {
		fmt.Fprintf(console.Debug(rootCtx), "Failed to set ulimit on number of open files to %d: %v\n", 4096, err)
	}

	var run *storedrun.Run

	rootCmd := newRoot(opts.Name, func(cmd *cobra.Command, args []string) error {
		if err := console.Prepare(); err != nil {
			return err
		}

		ctx := cmd.Context()

		// This is a bit of an hack. But don't run version checks when doing an update.
		if opts.NotifyOnNewVersion && !slices.Contains(cmd.Aliases, "update-ns") {
			DeferCheckVersion(ctx, opts.Name)
		}

		if viper.GetBool("enable_pprof") {
			go ListenPProf(console.Info(cmd.Context()))
		}

		run = storedrun.New()

		// Setting up container registry logging, which is unfortunately global.
		crlogs.Warn = log.New(console.TypedOutput(cmd.Context(), "cr-warn", idtypes.CatOutputTool), "", log.LstdFlags|log.Lmicroseconds)

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
	simplelog.SetupFlags(rootCmd.PersistentFlags())
	fnapi.SetupFlags(rootCmd.PersistentFlags())
	clerk.SetupFlags(rootCmd.PersistentFlags())

	rootCmd.PersistentFlags().BoolVar(&disableCommandBundle, "disable_command_bundle", disableCommandBundle,
		"If set to true, diagnostics and error information are disabled for the command and the command is filtered from `ns command-history`.")
	rootCmd.PersistentFlags().BoolVar(&console.DebugToConsole, "debug_to_console", console.DebugToConsole,
		"If set to true, we also output debug log messages to the console.")
	rootCmd.PersistentFlags().StringVar(&console.DebugToFile, "debug_to_file", "",
		"If set to true, outputs debug messages to the specified file.")
	rootCmd.PersistentFlags().BoolVar(&fnapi.DebugApiResponse, "debug_api_response", fnapi.DebugApiResponse,
		"If set to true, we also output debug log messages for API responses.")
	rootCmd.PersistentFlags().BoolVar(&enableErrorTracing, "error_tracing", enableErrorTracing,
		"If set to true, prints a trace of foundation errors leading to the root cause with source info.")

	storedrun.SetupFlags(rootCmd.PersistentFlags())

	knobs.SetupFlags(rootCmd.PersistentFlags())

	// We have too many flags, hide some of them from --help so users can focus on what's important.
	for _, noisy := range []string{
		"disable_command_bundle",
		"error_tracing",
		"debug_to_console",
		"debug_to_file",
		"debug_api_response",
	} {
		_ = rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	opts.RegisterCommands(rootCmd)

	debugLog := console.Debug(rootCtx)
	cmdCtx := tasks.ContextWithThrottler(rootCtx, debugLog, tasks.LoadThrottlerConfig(rootCtx, debugLog))

	err := RunInContext(cmdCtx, func(ctx context.Context) error {
		defer console.Cleanup()

		return rootCmd.ExecuteContext(ctx)
	})

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

	return style, err
}

func maybeRunLatest(rootCtx context.Context, style colors.Style, flushLogs func(), command string) {
	if ver, err := version.Current(); err == nil && !nsboot.SpawnedFromBoot() && version.ShouldCheckUpdate(ver) {
		if cached, ns, err := nsboot.UpdateIfNeeded(rootCtx, command, ver.Version); err == nil && cached != nil {
			flushLogs()

			ns.ExecuteAndForwardExitCode(rootCtx, style)
			// Never gets here.
		}
	}
}

func maybeRunLatestFromCache(rootCtx context.Context, style colors.Style, flushLogs func(), command string) {
	if ver, err := version.Current(); err == nil && !nsboot.SpawnedFromBoot() && version.ShouldUseCachedUpdate(ver) {
		if cached, ns, err := nsboot.CheckCachedUpdate(rootCtx, command, ver.Version); err == nil && cached != nil {
			flushLogs()

			ns.ExecuteAndForwardExitCode(rootCtx, style)
			// Never gets here.
		}
	}
}

type FormatErrorFunc func(io.Writer, colors.Style, error)

func handleExitError(style colors.Style, err error, formatError FormatErrorFunc) int {
	if exitError, ok := err.(fnerrors.ExitError); ok {
		// If we are exiting, because a sub-process failed, don't bother outputting
		// an error again, just forward the appropriate exit code.
		return exitError.ExitCode()
	} else {
		// Only print errors after calling flushLogs above, so the console driver
		// is no longer erasing lines.
		formatError(os.Stderr, style, err)
		return 1
	}
}

func DefaultErrorFormatter(out io.Writer, style colors.Style, err error) {
	format.Format(os.Stderr, err, format.WithStyle(style), format.WithTracing(enableErrorTracing), format.WithActionTrace(true))
}

func newRoot(name string, preRunE func(cmd *cobra.Command, args []string) error) *cobra.Command {
	root := &cobra.Command{
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

		// adds the welcome message to `ns`, `ns help` and `ns --help`
		Long: welcome.WelcomeMessage(false /* firstRun*/, name),
	}

	switch name {
	case "ns":
		root.Example = `  ns prepare local  Prepares the local workspace for development or production.
  ns test           Run all functional end-to-end tests in the current workspace.
  ns dev            Starts a development session, continuously building and deploying servers.`

	case "nsc":
		root.Example = `  nsc login    Log in to use Namespace Cloud.
  nsc create   Create a new cluster.
  nsc kubectl  Run kubectl in your cluster.
  nsc build    Build a Docker image in a build cluster.`
	}

	return root
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
	_ = viper.BindEnv("telemetry")

	viper.SetDefault("enable_autoupdate", true)
	_ = viper.BindEnv("enable_autoupdate")

	viper.SetDefault("console_log_level", 0)
	_ = viper.BindEnv("console_log_level")

	viper.SetDefault("enable_pprof", false)
	_ = viper.BindEnv("enable_pprof")

	viper.SetDefault("console_output_action_id", false)
	_ = viper.BindEnv("console_output_action_id")

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

		if err := os.MkdirAll(fnDir, 0o755); err == nil {
			if f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644); err == nil {
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

func consoleToSink(out *os.File, isTerm, inhibitReport bool, renderer consolesink.RendererFunc) (tasks.ActionSink, colors.Style, func()) {
	maxLogLevel := viper.GetInt("console_log_level")

	if filename, ok := os.LookupEnv("NS_LOG_TO_FILE"); ok {
		f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			log.Fatalf("could not open file %q: %v", filename, err)
		}

		return simplelog.NewSink(f, maxLogLevel), colors.NoColors, func() {
			f.Close()
		}
	}

	if isTerm && !viper.GetBool("console_no_colors") {
		consoleSink := consolesink.NewSink(out, consolesink.ConsoleSinkOpts{
			Interactive:   true,
			InhibitReport: inhibitReport,
			MaxLevel:      maxLogLevel,
			Renderer:      renderer,
		})
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

func setupContext(ctx context.Context, inhibitReport bool, rendered consolesink.RendererFunc) (context.Context, colors.Style, func()) {
	out, isterm := StandardConsole()
	sink, style, flushLogs := consoleToSink(out, isterm, inhibitReport, rendered)
	ctx = colors.WithStyle(tasks.WithSink(ctx, sink), style)
	if flushLogs == nil {
		flushLogs = func() {}
	}

	return ctx, style, flushLogs
}
