// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/docker/buildx/store"
	"github.com/docker/buildx/store/storeutil"
	"github.com/docker/buildx/util/dockerutil"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/muesli/reflow/wordwrap"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	metadataFile      = "metadata.json"
	defaultBuilder    = "nsc-remote"
	proxyDir          = "proxy"
	buildkitProxyPath = "buildkit/" + proxyDir
)

func newSetupBuildxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup buildx in the current machine, to use Namespace Remote builders.",
	}

	name := cmd.Flags().String("name", defaultBuilder, "The name of the builder we setup.")
	use := cmd.Flags().Bool("use", false, "If true, changes the current builder to nsc-remote.")
	background := cmd.Flags().Bool("background", false, "If true, runs the remote builder context in the background.")
	createAtStartup := cmd.Flags().Bool("create_at_startup", false, "If true, creates the build clusters eagerly.")
	stateDir := cmd.Flags().String("state", "", "If set, stores the remote builder context details in this directory.")
	debugDir := cmd.Flags().String("background_debug_dir", "", "If set with --background, the tool populates the specified directory with debug log files.")
	_ = cmd.Flags().MarkHidden("background_debug_dir")
	useGrpcProxy := cmd.Flags().Bool("use_grpc_proxy", true, "If set, traffic is proxied with transparent grpc proxy instead of raw network proxy")
	_ = cmd.Flags().MarkHidden("use_grpc_proxy")
	staticWorkerDefFile := cmd.Flags().String("static_worker_definition_path", "", "Injects the gRPC proxy ListWorkers response JSON payload from file")
	_ = cmd.Flags().MarkHidden("static_worker_definition_path")
	forceCleanup := cmd.Flags().Bool("force_cleanup", false, "If set, it forces a cleanup of any previous buildx proxy running in background.")
	waitForLogin := cmd.Flags().Bool("wait_for_login", false, "If set, it blocks waiting for user to login.")
	_ = cmd.Flags().MarkHidden("wait_for_login")
	annotateBuild := cmd.Flags().Bool("annotate_build", true, "If set, it enable builds annotation when running in Namespace instances.")
	_ = cmd.Flags().MarkHidden("annotate_build")
	buildkitSockPath := cmd.Flags().String("buildkit_sock_path", "", "If set, the proxy connect to a local unix socket rather than remote builder.")
	_ = cmd.Flags().MarkHidden("buildkit_sock_path")
	defaultLoad := cmd.Flags().Bool("default_load", false, "If true, load images to the Docker Engine image store if no other output is specified.")
	_ = cmd.Flags().MarkHidden("default_load")
	useServerSideProxy := cmd.Flags().Bool("use_server_side_proxy", false, "If set, buildx is setup to use transparent server-side proxy powered by Namespace")
	_ = cmd.Flags().MarkHidden("use_server_side_proxy")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *debugDir != "" && !*background {
			return fnerrors.Newf("--background_debug_dir requires --background")
		}

		if !*useGrpcProxy && *staticWorkerDefFile != "" {
			return fnerrors.Newf("--inject_worker_info_file requires --use_grpc_proxy")
		}

		// NSL-2249 for brew service, block here until user logs in.
		if *waitForLogin {
			err := blockWaitForLogin(ctx)
			if err != nil {
				return err
			}
		}

		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		available, serverSideProxyDefault, err := determineAvailable(ctx)
		if err != nil {
			return err
		}

		if len(available) == 0 {
			return rpcerrors.Errorf(codes.Internal, "no builders available")
		}

		state, err := ensureStateDir(*stateDir, buildkitProxyPath)
		if err != nil {
			return err
		}

		// NSL-3935 use remote-side buildx proxy
		// This will be soon the default
		if serverSideProxyDefault || *useServerSideProxy {
			if err := setupServerSideBuildxProxy(ctx, state, *name, *use, *defaultLoad, dockerCli, available, *createAtStartup); err != nil {
				return err
			}

			// Print info message.
			console.SetStickyContent(ctx, "build", banner(ctx, *name, *use, available, true, true))
			return nil
		}

		fmt.Fprintf(console.Debug(ctx), "Using state path %q\n", state)

		if proxyAlreadyExists(state) {
			if *forceCleanup {
				if err := withStore(dockerCli, func(txn *store.Txn) error {
					return doBuildxCleanup(ctx, state, txn)
				}); err != nil {
					console.SetStickyContent(ctx, "build", existingProxyMessage(*stateDir))
					return err
				}

				// Cleanup deletes also the state directory, recreate it
				state, err = ensureStateDir(*stateDir, buildkitProxyPath)
				if err != nil {
					return err
				}
			} else {
				console.SetStickyContent(ctx, "build", existingProxyMessage(*stateDir))
				return nil
			}
		}

		md := buildxMetadata{
			NodeGroupName: *name,
		}

		for _, p := range available {
			sockPath := filepath.Join(state, fmt.Sprintf("%s.sock", p))
			controlSockPath := filepath.Join(state, fmt.Sprintf("control_%s.sock", p))

			instanceMD := buildxInstanceMetadata{
				Platform:          p,
				SocketPath:        sockPath,
				Pid:               os.Getpid(), // This will be overwritten if running proxies in background
				ControlSocketPath: controlSockPath,
			}

			if *background {
				logFilename := fmt.Sprintf("%s-proxy.log", instanceMD.Platform)
				if *debugDir != "" {
					instanceMD.DebugLogPath = path.Join(*debugDir, logFilename)
				} else {
					logDir, err := ensureLogDir(proxyDir)
					if err != nil {
						return fnerrors.Newf("failed to create the log folder: %v", err)
					}

					instanceMD.DebugLogPath = path.Join(logDir, logFilename)
				}
			}

			md.Instances = append(md.Instances, instanceMD)
		}

		eg := executor.New(ctx, "proxies")
		var instances []BuildCluster
		for i, p := range md.Instances {
			// Always create one, in case it's needed below. This instance has a zero-ish cost if we never call NewConn.
			instance, err := NewBuildCluster(ctx, string(p.Platform), *buildkitSockPath)
			if err != nil {
				return fnerrors.Newf("failed to create builder: %w", err)
			}
			instances = append(instances, instance)

			if *background {
				if pid, err := startBackgroundProxy(ctx, p, *createAtStartup, *useGrpcProxy, *annotateBuild, *staticWorkerDefFile, *buildkitSockPath); err != nil {
					return err
				} else {
					md.Instances[i].Pid = pid
				}
			} else {
				workerInfoResp, err := parseInjectWorkerInfo(*staticWorkerDefFile, p.Platform)
				if err != nil {
					return fnerrors.Newf("failed to parse worker info JSON payload: %v", err)
				}

				bp, err := instance.RunBuildProxy(ctx, p.SocketPath, p.ControlSocketPath, *useGrpcProxy, *annotateBuild, workerInfoResp)
				if err != nil {
					return err
				}

				defer os.Remove(p.SocketPath)
				defer os.Remove(p.ControlSocketPath)

				eg.Go(func(ctx context.Context) error {
					<-ctx.Done()
					return bp.Cleanup()
				})

				eg.Go(func(ctx context.Context) error {
					return bp.Serve(ctx)
				})

				eg.Go(func(ctx context.Context) error {
					return bp.ServeStatus(ctx)
				})

				eg.Go(func(ctx context.Context) error {
					sigc := make(chan os.Signal, 1)
					signal.Notify(sigc, os.Interrupt, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGQUIT)
					select {
					case <-ctx.Done():
						fmt.Fprintf(console.Debug(ctx), "Ctx expired.\n")
					case <-sigc:
						fmt.Fprintf(console.Debug(ctx), "Received signal to exit.\n")
						eg.Cancel()
					}
					return nil
				})
			}
		}

		if err := files.WriteJson(filepath.Join(state, metadataFile), md, 0644); err != nil {
			return err
		}

		if *createAtStartup {
			eg := executor.New(ctx, "startup")

			for _, p := range instances {
				p := p // Close p
				eg.Go(func(ctx context.Context) error {
					_, _, err := p.NewConn(ctx)
					return err
				})
			}

			if err := eg.Wait(); err != nil {
				return err
			}
		}

		if err := wireBuildx(dockerCli, *name, *use, *defaultLoad, md); err != nil {
			return multierr.New(err, eg.CancelAndWait())
		}

		// Print info message even if proxy goes in background
		console.SetStickyContent(ctx, "build", banner(ctx, *name, *use, available, *background, false))

		if *background {
			return nil
		}

		if err := eg.Wait(); err != nil {
			return err
		}

		fmt.Fprintf(console.Debug(ctx), "Cleaning up docker buildx context.\n")
		if err := withStore(dockerCli, func(txn *store.Txn) error {
			return txn.Remove(*name)
		}); err != nil {
			return err
		}

		fmt.Fprintf(console.Debug(ctx), "Deleting state file.\n")
		if err := os.RemoveAll(state); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func proxyAlreadyExists(stateDir string) bool {
	_, err := os.Stat(filepath.Join(stateDir, metadataFile))
	return !os.IsNotExist(err)
}

func existingProxyMessage(customStateDir string) string {
	if customStateDir != "" {
		return fmt.Sprintf(`Previous remote builder configuration found in %s.
If you want to create a new remote builder context configuration, cleanup the older one first with:

   nsc docker buildx cleanup --state %s
`, customStateDir, customStateDir)
	} else {
		return `Previous remote builder configuration found.
If you want to create a new remote builder context configuration, cleanup the older one first with:

   nsc docker buildx cleanup
`
	}
}

func blockWaitForLogin(ctx context.Context) error {
	// Check immediately if token is valid by calling Namespace
	_, err := api.GetProfile(ctx, api.Methods)
	if err == nil {
		return err
	}

	// Else, block
	for {
		select {
		case <-time.After(time.Second * 5):
			_, err := api.GetProfile(ctx, api.Methods)
			if err == nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func ensureStateDir(specified, dir string) (string, error) {
	if specified == "" {
		// Change state dir from cache, which can be removed at any time,
		// to the app's config folder.
		// Older state dir might still be under the cache file, so we need to first check that path,
		// if it does not exist, we can create the new one, under config path.
		oldStateDirPath, err := dirs.Subdir(dir)
		if err != nil {
			return "", err
		}

		if proxyAlreadyExists(oldStateDirPath) {
			return oldStateDirPath, nil
		}

		return dirs.Ensure(dirs.ConfigSubdir(dir))
	}

	s, err := filepath.Abs(specified)
	if err != nil {
		return "", err
	}

	return dirs.Ensure(filepath.Join(s, proxyDir), nil)
}

// Returns the default state directory for nsc buildkit state.
// Otherwise returns "".
func getDefaultStateDirIfExists(dir string) (string, error) {
	// Mimic the logic of ensureStateDir but don't create directories.
	oldStateDirPath, err := dirs.Subdir(dir)
	if err != nil {
		return "", err
	}

	if proxyAlreadyExists(oldStateDirPath) {
		return oldStateDirPath, nil
	}

	newStateDirPath, err := dirs.ConfigSubdir(dir)
	if err != nil {
		return "", err
	}

	return newStateDirPath, nil
}

func ensureLogDir(dir string) (string, error) {
	return dirs.Ensure(dirs.Logs(dir))
}

type buildxMetadata struct {
	NodeGroupName string                   `json:"node_group_name"`
	Instances     []buildxInstanceMetadata `json:"instances"`
}

type buildxInstanceMetadata struct {
	Platform          api.BuildPlatform `json:"build_platform"`
	SocketPath        string            `json:"socket_path"`
	Pid               int               `json:"pid"`
	DebugLogPath      string            `json:"debug_log_path"`
	ControlSocketPath string            `json:"control_socket_path"`
}

func wireBuildx(dockerCli *command.DockerCli, name string, use, defaultLoad bool, md buildxMetadata) error {
	return withStore(dockerCli, func(txn *store.Txn) error {
		ng, err := txn.NodeGroupByName(name)
		if err != nil {
			if !os.IsNotExist(errors.Cause(err)) {
				return err
			}
		}

		const driver = "remote"

		if ng == nil {
			ng = &store.NodeGroup{
				Name:   name,
				Driver: driver,
			}
		}

		do := map[string]string{}

		if defaultLoad {
			// Supported starting with v0.14.0
			do["default-load"] = "true"
		}

		for _, p := range md.Instances {
			var platforms []string
			if p.Platform == "arm64" {
				platforms = []string{"linux/arm64"}
			}

			if err := ng.Update(string(p.Platform), "unix://"+p.SocketPath, platforms, true, true, nil, "", do); err != nil {
				return err
			}
		}

		if use {
			ep, err := dockerutil.GetCurrentEndpoint(dockerCli)
			if err != nil {
				return err
			}

			if err := txn.SetCurrent(ep, name, false, false); err != nil {
				return err
			}
		}

		if err := txn.Save(ng); err != nil {
			return err
		}

		return nil
	})
}

func newCleanupBuildxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Unregisters Namespace Remote builders from buildx.",
	}

	stateDir := cmd.Flags().String("state", "", "If set, looks for the remote builder context in this directory.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		return withStore(dockerCli, func(txn *store.Txn) error {
			state, err := ensureStateDir(*stateDir, buildkitProxyPath)
			if err != nil {
				return err
			}

			if !proxyAlreadyExists(state) {
				console.SetStickyContent(ctx, "build", "State file not found. Nothing to cleanup.")
				return nil
			}

			return doBuildxCleanup(ctx, state, txn)
		})
	})

	return cmd
}

func doBuildxCleanup(ctx context.Context, state string, txn *store.Txn) error {
	var md buildxMetadata
	if err := files.ReadJson(filepath.Join(state, metadataFile), &md); err != nil {
		return err
	}

	for _, inst := range md.Instances {
		if inst.Pid > 0 {
			process, err := os.FindProcess(inst.Pid)
			if err != nil {
				return err
			}

			if err := process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
				return err
			}

			fmt.Fprintf(console.Debug(ctx), "Sent SIGINT to worker handling %s (pid %d).\n", inst.Platform, inst.Pid)
		}
	}

	if err := os.RemoveAll(state); err != nil {
		console.SetStickyContent(ctx, "build",
			fmt.Sprintf("Warning: deleting state files in %s failed: %v", state, err))
	}

	fmt.Fprintf(console.Debug(ctx), "Removed local state directory %q.\n", state)

	if md.NodeGroupName != "" {
		if err := txn.Remove(md.NodeGroupName); err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Removed buildx node group %q.\n", md.NodeGroupName)
	}

	fmt.Fprintf(console.Stderr(ctx), "Cleanup complete.\n")
	return nil
}

func newWireBuildxCommand(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "wire",
		Short:  "Wires a previously setup proxy setup.",
		Hidden: hidden,
	}

	name := cmd.Flags().String("name", defaultBuilder, "The name of the builder we setup.")
	use := cmd.Flags().Bool("use", false, "If true, changes the current builder to nsc-remote.")
	stateDir := cmd.Flags().String("state", "", "Where the proxies live.")
	defaultLoad := cmd.Flags().Bool("default_load", false, "If true, load images to the Docker Engine image store if no other output is specified.")
	_ = cmd.Flags().MarkHidden("default_load")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *stateDir == "" {
			return fnerrors.Newf("--state is required")
		}

		dockerCli, err := command.NewDockerCli()
		if err != nil {
			return err
		}

		if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
			return err
		}

		var md buildxMetadata
		if err := files.ReadJson(filepath.Join(*stateDir, metadataFile), &md); err != nil {
			return err
		}

		return wireBuildx(dockerCli, *name, *use, *defaultLoad, md)
	})

	return cmd
}

func determineAvailable(ctx context.Context) ([]api.BuildPlatform, bool, error) {
	profile, err := api.GetProfile(ctx, api.Methods)
	if err != nil {
		return nil, false, err
	}

	avail := make([]api.BuildPlatform, len(profile.ClusterPlatform))
	for k, x := range profile.ClusterPlatform {
		avail[k] = api.BuildPlatform(x)
	}

	return avail, profile.BuildxServerSideProxyDefaultHint, nil
}

func banner(ctx context.Context, name string, use bool, native []api.BuildPlatform, background, serverSideProxy bool) string {
	w := wordwrap.NewWriter(80)
	style := colors.Ctx(ctx)

	fmt.Fprint(w, style.Highlight.Apply("docker buildx"), " has been configured to use ",
		style.Highlight.Apply("Namespace Remote builders"), ".\n")

	fmt.Fprintln(w)
	fmt.Fprint(w, "Native building: ", strings.Join(bold(style, native), " and "), ".\n")

	if !use {
		fmt.Fprint(w, "\nThe default buildx builder was not changed, you can re-run with ", style.Highlight.Apply("--use"), " or run:\n")
		fmt.Fprintf(w, "\n  docker buildx use %s\n", name)
	}

	if !background {
		fmt.Fprintf(w, "\nStart a new terminal, and start building:\n")
		fmt.Fprintf(w, "\n  docker buildx build ...\n")

		fmt.Fprintln(w)
		fmt.Fprintln(w, style.Comment.Apply("Exiting will remove the configuration."))
	} else {
		fmt.Fprintf(w, "\nStart building:\n")
		fmt.Fprintf(w, "\n  docker buildx build ...\n")

		if serverSideProxy {
			fmt.Fprintf(w, "\nYou can remove the configuration with:\n")
			fmt.Fprintf(w, "\n  nsc docker buildx cleanup \n")
		} else {
			fmt.Fprintf(w, "\nYour remote builder context is running in the background. You can always clean it up with:\n")
			fmt.Fprintf(w, "\n  nsc docker buildx cleanup \n")
		}
	}

	_ = w.Close()

	return strings.TrimSpace(w.String())
}

func bold[X any](style colors.Style, values []X) []string {
	result := make([]string, len(values))
	for k, x := range values {
		result[k] = style.Highlight.Apply(fmt.Sprintf("%v", x))
	}
	return result
}

func withStore(dockerCli *command.DockerCli, f func(*store.Txn) error) error {
	txn, release, err := storeutil.GetStore(dockerCli)
	if err != nil {
		return err
	}
	defer release()

	return f(txn)
}

func buildxContextNotConfigured() string {
	return `Docker buildx context is not configured for Namespace remote builders.
Try running:

   nsc docker buildx setup --use --background
`
}

func buildxContextNotRunning() string {
	return `It seems that Namespace buildx context is not running.
Try running the following to restart it:

   nsc docker buildx cleanup && nsc docker buildx setup --use --background
`
}

func makeUnixHTTPClient(unixSockPath string) *http.Client {
	unixDial := func(proto, addr string) (conn net.Conn, err error) {
		return net.Dial("unix", unixSockPath)
	}

	tr := &http.Transport{
		Dial: unixDial,
	}

	return &http.Client{Transport: tr}
}

func newStatusBuildxCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Status information for the local Namespace buildx context.",
	}

	output := cmd.Flags().StringP("output", "o", "plain", "One of plain or json.")
	stateDir := cmd.Flags().String("state", "", "If set, looks for the remote builder context in this directory.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		state, err := ensureStateDir(*stateDir, buildkitProxyPath)
		if err != nil {
			return err
		}

		var md buildxMetadata
		if err := files.ReadJson(filepath.Join(state, metadataFile), &md); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				console.SetStickyContent(ctx, "build", buildxContextNotConfigured())
				return nil
			}
			return err
		}

		descs := []StatusData{}
		for _, proxy := range md.Instances {
			client := makeUnixHTTPClient(proxy.ControlSocketPath)
			resp, err := client.Get("http://localhost/status")
			if err != nil {
				console.SetStickyContent(ctx, "build", buildxContextNotRunning())
				return err
			}
			defer resp.Body.Close()

			var desc StatusData
			dec := json.NewDecoder(resp.Body)
			if err := dec.Decode(&desc); err != nil {
				return err
			}

			descs = append(descs, desc)
		}

		stdout := console.Stdout(ctx)
		switch *output {
		case "json":
			enc := json.NewEncoder(console.Stdout(ctx))
			enc.SetIndent("", "  ")
			if err := enc.Encode(descs); err != nil {
				return fnerrors.InternalError("failed to encode status as JSON output: %w", err)
			}

		default:
			if *output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "defaulting output to plain\n")
			}

			fmt.Fprintf(stdout, "\nBuildx context status:\n\n")
			for _, desc := range descs {
				fmt.Fprintf(stdout, "Platform: %s\n", desc.Platform)
				fmt.Fprintf(stdout, "  Status: %s\n", desc.Status)
				fmt.Fprintf(stdout, "  Last Instance ID: %s\n", desc.LastInstanceID)
				fmt.Fprintf(stdout, "  Last Update: %v\n", desc.LastUpdate)
				fmt.Fprintf(stdout, "  Last Error: %v\n", desc.LastError)
				fmt.Fprintf(stdout, "  Requests Handled: %v\n", desc.Requests)
				fmt.Fprintf(stdout, "  Log Path: %v\n", desc.LogPath)
				fmt.Fprintf(stdout, "\n")
			}
		}

		return nil
	})

	return cmd
}
