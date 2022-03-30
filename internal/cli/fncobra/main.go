// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncobra

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/logs"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	clrs "namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/fscache"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/logoutput"
	"namespacelabs.dev/foundation/internal/sdk/k3d"
	"namespacelabs.dev/foundation/languages/golang"
	"namespacelabs.dev/foundation/languages/nodejs"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/languages/web"
	ecr "namespacelabs.dev/foundation/providers/aws/registry"
	artifactregistry "namespacelabs.dev/foundation/providers/gcp/registry"
	"namespacelabs.dev/foundation/provision/deploy"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/dirs"
	src "namespacelabs.dev/foundation/workspace/source"
	"namespacelabs.dev/foundation/workspace/source/codegen"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func DoMain(name string, registerCommands func(*cobra.Command)) {
	if v := os.Getenv("FN_CPU_PROFILE"); v != "" {
		done := cpuprofile(v)
		defer done()
	}

	setupViper()

	ctx := context.Background()

	var cleanupTracer func()
	if tracerEndpoint := viper.GetString("jaeger_endpoint"); tracerEndpoint != "" && viper.GetBool("enable_tracing") {
		ctx, cleanupTracer = tasks.SetupTracing(ctx, tracerEndpoint)
	}

	out, colors := consoleFromFile()

	logger, sink, flushLogs := consoleToSink(out, colors)

	tagNameChannel := make(chan string)
	go checkForUpdates(logger, tagNameChannel)

	ctxWithSink := tasks.WithSink(logger.WithContext(ctx), sink)

	tel := fnapi.NewTelemetry()

	var storeActions bool

	rootCmd := newRoot(name, func(cmd *cobra.Command, args []string) error {
		if storeActions {
			var err error
			tasks.ActionStorer, err = tasks.NewStorer(cmd.Context())
			if err != nil {
				return err
			}
		}

		// Used for devhost/environment validation.
		devhost.HasRuntime = func(w *schema.Workspace, e *schema.Environment, dh *schema.DevHost) bool {
			return runtime.ForProto(w, e, dh) != nil
		}

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

		// Setting up container registry logging, which is unfortunately global.
		logs.Warn = log.New(console.TypedOutput(cmd.Context(), "cr-warn", tasks.CatOutputTool), "", log.LstdFlags|log.Lmicroseconds)

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
		artifactregistry.Register()
		oci.RegisterDomainKeychain("pkg.dev", artifactregistry.DefaultKeychain)
		oci.RegisterDomainKeychain("amazonaws.com", ecr.DefaultKeychain)

		// Runtimes.
		kubernetes.Register()
		kubernetes.RegisterGraphHandlers()

		// Telemetry.
		tel.RecordInvocation(ctxWithSink, cmd, args)
		return nil
	})

	registerCommands(rootCmd)

	rootCmd.PersistentFlags().BoolVar(&binary.UsePrebuilts, "use_prebuilts", binary.UsePrebuilts,
		"If set to false, binaries are built from source rather than a corresponding prebuilt being used.")
	rootCmd.PersistentFlags().BoolVar(&tasks.LogActions, "log_actions", tasks.LogActions,
		"If set to true, each completed action is also output as a log message.")
	rootCmd.PersistentFlags().BoolVar(&compute.CachingEnabled, "caching", compute.CachingEnabled,
		"If set to false, compute caching is disabled.")
	rootCmd.PersistentFlags().BoolVar(&storeActions, "store_actions", storeActions,
		"If set to true, each completed action and its attachments are also persisted into storage.")

	rootCmd.PersistentFlags().Var(buildkit.ImportCacheVar, "buildkit_import_cache", "Internal, set buildkit import-cache.")
	rootCmd.PersistentFlags().Var(buildkit.ExportCacheVar, "buildkit_export_cache", "Internal, set buildkit export-cache.")
	rootCmd.PersistentFlags().BoolVar(&golang.UseBuildKitForBuilding, "golang_use_buildkit", golang.UseBuildKitForBuilding,
		"If set to true, buildkit is used for building, instead of a ko-style builder.")
	rootCmd.PersistentFlags().BoolVar(&deploy.AlsoComputeIngress, "also_compute_ingress", deploy.AlsoComputeIngress,
		"[development] Set to false, to skip ingress computation.")
	rootCmd.PersistentFlags().BoolVar(&tel.UseTelemetry, "send_usage_data", tel.UseTelemetry,
		"If set to false, fn does not upload any usage data.")
	rootCmd.PersistentFlags().BoolVar(&buildkit.SkipExpectedMaxWorkspaceSizeCheck, "skip_buildkit_workspace_size_check", buildkit.SkipExpectedMaxWorkspaceSizeCheck,
		"If set to true, skips our enforcement of the maximum workspace size we're willing to push to buildkit.")
	rootCmd.PersistentFlags().BoolVar(&k3d.IgnoreZfsCheck, "ignore_zfs_check", k3d.IgnoreZfsCheck,
		"If set to true, ignores checkign whether the base system is ZFS based.")

	// We have too many flags, hide some of them from --help so users can focus on what's important.
	for _, noisy := range []string{
		"buildkit_import_cache",
		"buildkit_export_cache",
		"also_compute_ingress",
		"golang_use_buildkit",
		"send_usage_data",
		"telemetry_error_logging",
		"skip_buildkit_workspace_size_check",
		"ignore_zfs_check",
	} {
		rootCmd.PersistentFlags().MarkHidden(noisy)
	}

	err := rootCmd.ExecuteContext(ctxWithSink)

	if flushLogs != nil {
		flushLogs()
	}

	if cleanupTracer != nil {
		cleanupTracer()
	}

	if tasks.ActionStorer != nil {
		tasks.ActionStorer.Flush(os.Stderr)
	}

	// Printing the new version message if any.
	select {
	case tagName := <-tagNameChannel:
		msg := fmt.Sprintf(
			"New Foundation release %s is available.\nDownload: https://github.com/namespacelabs/foundation/releases/tag/%s",
			tagName, tagName)
		if colors {
			fmt.Fprintln(console.Stdout(ctx), clrs.Green(msg))
		} else {
			fmt.Fprintln(console.Stdout(ctx), msg)
		}
	default:
	}

	if err != nil {
		exitCode := 1
		if exitError, ok := err.(*exec.ExitError); ok {
			// If we are exiting, because a sub-process failed, don't bother outputting
			// an error again, just forward the appropriate exit code.
			exitCode = exitError.ExitCode()
		} else {
			// Only print errors after calling flushLogs above, so the console driver
			// is no longer erasing lines.
			fnerrors.Format(os.Stderr, colors, err)
			exitCode = 1
		}

		// Record errors only after the user sees them to hide potential latency implications.
		// We pass the original ctx without sink since logs have already been flushed.
		tel.RecordError(ctx, err)
		os.Exit(exitCode)
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
	viper.BindEnv("log_level")

	viper.SetDefault("jaeger_endpoint", "")
	viper.BindEnv("jaeger_endpoint")

	viper.SetDefault("console_output", "text")
	viper.BindEnv("console_output")

	viper.SetDefault("console_no_colors", false)
	viper.BindEnv("console_no_colors")

	viper.SetDefault("enable_tracing", false)
	viper.BindEnv("enable_tracing")

	viper.SetDefault("enable_telemetry", true)
	viper.SetDefault("telemetry_salt", "")
	viper.SetDefault("telemetry_user_id", "")

	viper.SetDefault("console_log_level", 0)
	viper.BindEnv("console_log_level")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Fatal(err)
		}
	}
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
	out := os.Stdout
	return out, termios.IsTerm(out.Fd())
}

func outputType() logoutput.OutputType {
	if strings.ToLower(viper.GetString("console_output")) == "json" {
		return logoutput.OutputJSON
	}
	return logoutput.OutputText
}

func consoleToSink(out *os.File, colors bool) (*zerolog.Logger, tasks.ActionSink, func()) {
	logout := logoutput.OutputTo{Writer: out, WithColors: colors, OutputType: outputType()}

	if colors && !viper.GetBool("console_no_colors") {
		consoleSink := tasks.NewConsoleSink(out, viper.GetInt("console_log_level"))
		cleanup := consoleSink.Start()
		logout.Writer = console.ConsoleOutputWith(consoleSink, tasks.KnownStderr)

		return logout.Logger(), consoleSink, cleanup
	}

	logger := logout.Logger()

	return logger, tasks.NewLoggerSink(logger), nil
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

// Does nothing if version check failed
func checkForUpdates(logger *zerolog.Logger, tagNameChannel chan string) {
	defer close(tagNameChannel)

	tagName, latestReleaseBuildTime, err := getLatestReleaseVersion()
	if err != nil {
		logger.Debug().AnErr("latest_release_version", err).Msg("version check failed")
		return
	}
	logger.Debug().Str("latest_release_version", latestReleaseBuildTime.String()).Msg("version check")

	binaryBuildTime, err := getBinaryBuildTime()
	if err != nil {
		logger.Debug().AnErr("binary_build_time", err).Msg("version check failed")
		return
	}
	if binaryBuildTime == nil {
		logger.Debug().Str("modified_build", "true").Msg("version check")
		return
	}
	logger.Debug().Str("binary_build_time", binaryBuildTime.String()).Msg("version check")

	if latestReleaseBuildTime.After(*binaryBuildTime) {
		tagNameChannel <- *tagName
	}
}

type version struct {
	TagName   string `json:"tag_name"`
	CreatedAt string `json:"created_at"`
}

func getLatestReleaseVersion() (tagName *string, createdAt *time.Time, err error) {
	response, err := http.Get("https://namespacelabs.github.io/foundation-version/version.json")
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, nil, err
	}

	var latestVersion version
	err = json.Unmarshal(body, &latestVersion)
	if err != nil {
		return nil, nil, err
	}
	latestBuildTime, err := time.Parse(time.RFC3339, latestVersion.CreatedAt)
	return &latestVersion.TagName, &latestBuildTime, err
}

// Returns nil if this "fn" was built manually.
func getBinaryBuildTime() (*time.Time, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fnerrors.InternalError("buildinfo is missing")
	}

	for _, n := range info.Settings {
		if n.Key == "vcs.modified" {
			// Ignore manual builds
			return nil, nil
		}
	}
	for _, n := range info.Settings {
		if n.Key == "vcs.time" {
			buildTime, err := time.Parse(time.RFC3339, n.Value)
			if err != nil {
				return nil, err
			}
			return &buildTime, nil
		}
	}

	return nil, fnerrors.InternalError("Couldn't find 'vcs.time' in buildinfo")
}
