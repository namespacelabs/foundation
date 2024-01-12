// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	types "github.com/moby/buildkit/api/types"
	"github.com/moby/buildkit/client"
	"github.com/spf13/cobra"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewBuildkitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "buildkit",
		Short:  "Buildkit-related functionality.",
		Hidden: true,
	}

	cmd.AddCommand(newBuildctlCmd())
	cmd.AddCommand(newBuildkitProxy())

	return cmd
}

func newBuildctlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildctl -- ...",
		Short: "Run buildctl on the target build cluster.",
	}

	buildCluster := cmd.Flags().String("cluster", "", "Set the type of a the build cluster to use.")
	platform := cmd.Flags().String("platform", "amd64", "One of amd64 or arm64.")

	cmd.Flags().MarkDeprecated("cluster", "use --platform instead")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *buildCluster != "" && *platform != "" {
			return fnerrors.New("--cluster and --platform are exclusive")
		}

		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		var plat api.BuildPlatform
		if *platform != "" {
			p, err := api.ParseBuildPlatform(*platform)
			if err != nil {
				return err
			}
			plat = p
		} else {
			if p, ok := compatClusterIDAsBuildPlatform(buildClusterOrDefault(*buildCluster)); ok {
				plat = p
			} else {
				return fnerrors.New("expected --cluster=build-cluster or build-cluster-arm64")
			}
		}

		p, err := runBuildProxyWithRegistry(ctx, plat, false, false, false, nil)
		if err != nil {
			return err
		}

		defer p.Cleanup()

		return runBuildctl(ctx, buildctlBin, p, args...)
	})

	return cmd
}

func newBuildkitProxy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run a platform-specific buildkit proxy.",
		Args:  cobra.NoArgs,
	}

	sockPath := cmd.Flags().String("sock_path", "", "If specified listens on the specified path.")
	platform := cmd.Flags().String("platform", "amd64", "One of amd64, or arm64.")
	background := cmd.Flags().String("background", "", "If specified runs the proxy in the background, and writes the process PID to the specified path.")
	createAtStartup := cmd.Flags().Bool("create_at_startup", false, "If true, eagerly creates the build clusters.")
	useGrpcProxy := cmd.Flags().Bool("use_grpc_proxy", true, "If set, traffic is proxied with transparent grpc proxy instead of raw network proxy.")
	_ = cmd.Flags().MarkHidden("use_grpc_proxy")
	staticWorkerDefFile := cmd.Flags().String("static_worker_definition_path", "", "Injects the gRPC proxy ListWorkers response JSON payload from file")
	_ = cmd.Flags().MarkHidden("static_worker_definition_path")
	controlSockPath := cmd.Flags().String("control_sock_path", "", "If set, status HTTP server listens to this unix socket path.")
	_ = cmd.Flags().MarkHidden("control_sock_path")
	annotateBuild := cmd.Flags().Bool("annotate_build", false, "If set, annotate builds when running in Namespace instances.")
	_ = cmd.Flags().MarkHidden("annotate_build")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		plat, err := api.ParseBuildPlatform(*platform)
		if err != nil {
			return err
		}

		if !*useGrpcProxy && *staticWorkerDefFile != "" {
			return fnerrors.New("--inject_worker_info requires --use_grpc_proxy")
		}

		if *background != "" {
			if *sockPath == "" {
				return fnerrors.New("--background requires --sock_path")
			}

			pid, err := startBackgroundProxy(ctx, buildxInstanceMetadata{SocketPath: *sockPath, Platform: plat, ControlSocketPath: *controlSockPath}, *createAtStartup, *useGrpcProxy, *annotateBuild, *staticWorkerDefFile)
			if err != nil {
				return err
			}

			return os.WriteFile(*background, []byte(fmt.Sprintf("%d", pid)), 0644)
		}

		workerInfoResp, err := parseInjectWorkerInfo(*staticWorkerDefFile, plat)
		if err != nil {
			return fnerrors.New("failed to parse worker info JSON payload: %v", err)
		}

		bp, err := runBuildProxy(ctx, plat, *sockPath, *controlSockPath, *createAtStartup, *useGrpcProxy, *annotateBuild, workerInfoResp)
		if err != nil {
			return err
		}

		eg := executor.New(ctx, "proxy")

		eg.Go(func(ctx context.Context) error {
			<-ctx.Done()
			return bp.Cleanup()
		})

		eg.Go(func(ctx context.Context) error {
			return bp.Serve(ctx)
		})

		if *controlSockPath != "" {
			eg.Go(func(ctx context.Context) error {
				return bp.ServeStatus(ctx)
			})
			fmt.Fprintf(console.Stderr(ctx), "Status server listening on %s\n", bp.controlSocketPath)
		}

		fmt.Fprintf(console.Stderr(ctx), "Listening on %s\n", bp.socketPath)

		if err := eg.Wait(); err != nil {
			return err
		}

		return nil
	})

	return cmd
}

func parseInjectWorkerInfo(workerInfoFile string, requiredPlatform api.BuildPlatform) (*controlapi.ListWorkersResponse, error) {
	var workerDefData []byte
	if workerInfoFile != "" {
		workerInfo, err := os.ReadFile(workerInfoFile)
		if err != nil {
			return nil, err
		}
		workerDefData = workerInfo
	} else {
		// Settlemint uses the buildx also for local dev, but tenant token expires after 24h.
		// API returns an authz error, but buildx does not show the error unless we unstuck it from its
		// known bug where it block on ListWorkers buildkit API call for ever.
		// So, embed the shortcut list worker payload:
		workerDefData = []byte(staticWorkerDef)
	}

	f := &controlapi.ListWorkersResponse{}
	if err := json.Unmarshal(workerDefData, f); err != nil {
		return nil, err
	}

	// Include only the worker definitions that include *at least* one
	// platform matching the proxy's (e.g. [arm64,amd64] worker matches for arm64 proxy)
	newRecords := []*types.WorkerRecord{}
	for _, r := range f.Record {
	platformLoop:
		for _, plat := range r.Platforms {
			if plat.Architecture == string(requiredPlatform) {
				newRecords = append(newRecords, r)
				break platformLoop
			}
		}
	}

	f.Record = newRecords
	return f, nil
}

func startBackgroundProxy(ctx context.Context, md buildxInstanceMetadata, connect bool, useGrpcProxy, annotateBuild bool, staticWorkerDefFile string) (int, error) {
	if connect {
		// Make sure the cluster exists before going to the background.
		if _, err := ensureBuildCluster(ctx, md.Platform); err != nil {
			return 0, err
		}
	}

	cmd := exec.Command(os.Args[0], "buildkit", "proxy", "--sock_path="+md.SocketPath,
		"--platform="+string(md.Platform), "--region="+api.RegionName, "--control_sock_path="+md.ControlSocketPath)
	if md.DebugLogPath != "" {
		cmd.Args = append(cmd.Args, "--debug_to_file="+md.DebugLogPath)
	}

	if useGrpcProxy {
		cmd.Args = append(cmd.Args, "--use_grpc_proxy")
		if staticWorkerDefFile != "" {
			cmd.Args = append(cmd.Args, "--static_worker_definition_path", staticWorkerDefFile)
		}
		if annotateBuild {
			cmd.Args = append(cmd.Args, "--annotate_build")
		}
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	fmt.Fprintf(console.Debug(ctx), "Running background command %q\n", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		return -1, err
	}

	pid := cmd.Process.Pid
	// Make sure the child process is not cleaned up on exit.
	if err := cmd.Process.Release(); err != nil {
		return -1, err
	}

	return pid, nil
}

func runBuildctl(ctx context.Context, buildctlBin buildctl.Buildctl, p *buildProxyWithRegistry, args ...string) error {
	cmdLine := []string{"--addr", "unix://" + p.BuildkitAddr}
	cmdLine = append(cmdLine, args...)

	fmt.Fprintf(console.Debug(ctx), "buildctl %s\n", strings.Join(cmdLine, " "))

	buildctl := exec.CommandContext(ctx, string(buildctlBin), cmdLine...)
	buildctl.Env = os.Environ()
	buildctl.Env = append(buildctl.Env, fmt.Sprintf("DOCKER_CONFIG="+p.DockerConfigDir))

	return localexec.RunInteractive(ctx, buildctl)
}

func ensureBuildCluster(ctx context.Context, platform api.BuildPlatform) (*api.CreateClusterResult, error) {
	response, err := api.CreateBuildCluster(ctx, api.Methods, platform)
	if err != nil {
		return nil, err
	}

	if err := waitUntilReady(ctx, response); err != nil {
		return nil, err
	}

	return response, nil
}

func resolveBuildkitService(response *api.CreateClusterResult) (*api.Cluster_ServiceState, error) {
	if override := os.Getenv("NSC_BUILDKIT_ENDPOINT_ADDRESS_OVERRIDE"); override != "" {
		return &api.Cluster_ServiceState{Endpoint: override}, nil
	}

	buildkitSvc := api.ClusterService(response.Cluster, "buildkit")
	if buildkitSvc == nil || buildkitSvc.Endpoint == "" {
		return nil, fnerrors.New("cluster is missing buildkit")
	}

	if buildkitSvc.Status != "READY" {
		return nil, fnerrors.New("expected buildkit to be READY, saw %q", buildkitSvc.Status)
	}

	return buildkitSvc, nil
}

func waitUntilReady(ctx context.Context, response *api.CreateClusterResult) error {
	buildkitSvc, err := resolveBuildkitService(response)
	if err != nil {
		return err
	}

	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		// We must fetch a token with our parent context, so we get a task sink etc.
		token, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		return buildkitfw.WaitReadiness(ctx, 1*time.Minute, func(innerCtx context.Context) (*client.Client, error) {
			return client.New(innerCtx, response.ClusterId, client.WithContextDialer(func(innerCtx context.Context, _ string) (net.Conn, error) {
				return api.DialEndpointWithToken(innerCtx, token, buildkitSvc.Endpoint)
			}))
		})
	})
}

func buildClusterOrDefault(bp string) string {
	if bp == "" {
		return buildCluster
	}
	return bp
}
