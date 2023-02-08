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
	crlogs "github.com/google/go-containerregistry/pkg/logs"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/clerk"
	"namespacelabs.dev/foundation/internal/cli/nsboot"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/console/consolesink"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/format"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/internal/ulimit"
	"namespacelabs.dev/foundation/internal/welcome"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tasks/actiontracing"
	"namespacelabs.dev/foundation/std/tasks/simplelog"
)

var (
	enableErrorTracing   = false
	disableCommandBundle = false
)

func DoMain(name string, autoUpdate bool, registerCommands func(*cobra.Command)) {
	style, err := doMain(name, autoUpdate, registerCommands)

	if err != nil && !errors.Is(err, context.Canceled) {
		os.Exit(handleExitError(style, err))
	}
}

func doMain(name string, autoUpdate bool, registerCommands func(*cobra.Command)) (colors.Style, error) {
	if v := os.Getenv("FN_CPU_PROFILE"); v != "" {
		done := cpuprofile(v)
		defer done()
	}

	SetupViper()

	// These are required for nsboot.
	compute.RegisterProtoCacheable()
	compute.RegisterBytesCacheable()
	fscache.RegisterFSCacheable()

	// Before moving forward, we check if there's a more up-to-date ns we should fork to.
	if autoUpdate {
		ensureLatest()
	}

	rootCtx, style, flushLogs := SetupContext(context.Background())

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

	rootCmd := newRoot(name, func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// This is a bit of an hack. But don't run version checks when doing an update.
		if autoUpdate && !slices.Contains(cmd.Aliases, "update-ns") {
			DeferCheckVersion(ctx)
		}

		tel := fnapi.TelemetryOn(ctx)

		// XXX move id management out of telemetry, it's used for other purposes too.
		if tel.IsFirstRun() && !environment.IsRunningInCI() {
			// First NS run - print a welcome message.
			welcome.PrintWelcome(ctx, true /* firstRun */, name)
		}

		// Now that "useTelemetry" flag is parsed, we can conditionally enable telemetry.
		if useTelemetry {
			tel.Enable()
		}

		if viper.GetBool("enable_pprof") {
			go ListenPProf(console.Debug(cmd.Context()))
		}

		run = storedrun.New()

		// Setting up container registry logging, which is unfortunately global.
		crlogs.Warn = log.New(console.TypedOutput(cmd.Context(), "cr-warn", common.CatOutputTool), "", log.LstdFlags|log.Lmicroseconds)

		// Telemetry.
		tel.RecordInvocation(ctx, cmd, args)

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
	clerk.SetupFlags(rootCmd.PersistentFlags())

	rootCmd.PersistentFlags().BoolVar(&disableCommandBundle, "disable_command_bundle", disableCommandBundle,
		"If set to true, diagnostics and error information are disabled for the command and the command is filtered from `ns command-history`.")
	rootCmd.PersistentFlags().BoolVar(&console.DebugToConsole, "debug_to_console", console.DebugToConsole,
		"If set to true, we also output debug log messages to the console.")
	rootCmd.PersistentFlags().BoolVar(&useTelemetry, "send_usage_data", true,
		"If set to false, ns does not upload any usage data.")
	rootCmd.PersistentFlags().BoolVar(&enableErrorTracing, "error_tracing", enableErrorTracing,
		"If set to true, prints a trace of foundation errors leading to the root cause with source info.")

	storedrun.SetupFlags(rootCmd.PersistentFlags())

	knobs.SetupFlags(rootCmd.PersistentFlags())

	// We have too many flags, hide some of them from --help so users can focus on what's important.
	for _, noisy := range []string{
		"disable_command_bundle",
		"send_usage_data",
		"error_tracing",
		"debug_to_console",
	} {
		_ = rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	registerCommands(rootCmd)

	debugLog := console.Debug(rootCtx)
	cmdCtx := tasks.ContextWithThrottler(rootCtx, debugLog, tasks.LoadThrottlerConfig(rootCtx, debugLog))

	err := RunInContext(cmdCtx, func(ctx context.Context) error {
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

	if err != nil && !errors.Is(err, context.Canceled) {
		if tel := fnapi.TelemetryOn(rootCtx); tel != nil {
			// Record errors only after the user sees them to hide potential latency implications.
			// We pass the original ctx without sink since logs have already been flushed.
			tel.RecordError(rootCtx, err)
		}

		return style, err
	}

	return style, err
}

func ensureLatest() {
	if ver, err := version.Current(); err == nil {
		if !nsboot.SpawnedFromBoot() && version.ShouldCheckUpdate(ver) {
			rootCtx, style, flushLogs := SetupContext(context.Background())

			cached, ns, err := nsboot.CheckUpdate(rootCtx, true, ver.Version)
			if err == nil && cached != nil {
				flushLogs()

				ns.ExecuteAndForwardExitCode(rootCtx, style)
				// Never gets here.
			}
		}
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
		root.Example = `  nsc login            Log in to use Namespace Cloud.
  nsc cluster create   Create a new cluster.
  nsc cluster kubectl  Run kubectl in your cluster.`
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
