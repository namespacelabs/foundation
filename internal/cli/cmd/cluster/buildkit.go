// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/moby/buildkit/client"
	"github.com/spf13/cobra"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
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

		p, err := runBuildProxyWithRegistry(ctx, plat, false, false)
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
	createAtStartup := cmd.Flags().Bool("create_at_startup", true, "If true, eagerly creates the build clusters.")
	useGrpcProxy := cmd.Flags().Bool("use_grpc_proxy", true, "If set, traffic is proxied with transparent grpc proxy instead of raw network proxy.")
	_ = cmd.Flags().MarkHidden("use_grpc_proxy")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		plat, err := api.ParseBuildPlatform(*platform)
		if err != nil {
			return err
		}

		if *background != "" {
			if *sockPath == "" {
				return fnerrors.New("--background requires --sock_path")
			}

			pid, err := startBackgroundProxy(ctx, buildxInstanceMetadata{SocketPath: *sockPath, Platform: plat}, *createAtStartup, "", *useGrpcProxy)
			if err != nil {
				return err
			}

			return os.WriteFile(*background, []byte(fmt.Sprintf("%d", pid)), 0644)
		}

		bp, err := runBuildProxy(ctx, plat, *sockPath, *createAtStartup, *useGrpcProxy)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Listening on %s\n", bp.socketPath)

		defer bp.Cleanup()

		return bp.Serve(ctx)
	})

	return cmd
}

func startBackgroundProxy(ctx context.Context, md buildxInstanceMetadata, connect bool, debugFile string, useGrpcProxy bool) (int, error) {
	if connect {
		// Make sure the cluster exists before going to the background.
		if _, err := ensureBuildCluster(ctx, md.Platform); err != nil {
			return 0, err
		}
	}

	cmd := exec.Command(os.Args[0], "buildkit", "proxy", "--sock_path="+md.SocketPath, "--platform="+string(md.Platform), "--region="+api.RegionName)
	if debugFile != "" {
		cmd.Args = append(cmd.Args, "--debug_to_file="+debugFile)
	}

	if useGrpcProxy {
		cmd.Args = append(cmd.Args, "--use_grpc_proxy")
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
		return buildkitfw.WaitReadiness(ctx, func() (*client.Client, error) {
			// We must fetch a token with our parent context, so we get a task sink etc.
			token, err := fnapi.FetchToken(ctx)
			if err != nil {
				return nil, err
			}

			return client.New(ctx, response.ClusterId, client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialEndpointWithToken(ctx, token, buildkitSvc.Endpoint)
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
