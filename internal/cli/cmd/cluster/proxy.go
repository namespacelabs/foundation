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
	"syscall"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const (
	buildCluster      = "build-cluster"
	buildClusterArm64 = "build-cluster-arm64"
)

func NewProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "proxy",
		Short:  "Runs a unix socket proxy for a well known service.",
		Hidden: true,
		Args:   cobra.NoArgs,
	}

	kind := cmd.Flags().String("kind", "", "The service being proxied.")
	sockPath := cmd.Flags().String("sock_path", "", "If specified listens on the specified path.")
	cluster := cmd.Flags().String("cluster", "", "Cluster ID to proxy; or 'build-cluster' and 'build-cluster-arm64' to proxy the build cluster.")
	background := cmd.Flags().String("background", "", "If specified runs the proxy in the background, and writes the process PID to the specified path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *cluster == "" || *kind == "" {
			return fnerrors.New("--cluster and --kind are required")
		}

		if *background != "" {
			if *sockPath == "" {
				return fnerrors.New("--background requires --sock_path")
			}

			// Make sure the cluster exists before going to the background.
			resolved, err := establishCluster(ctx, *cluster)
			if err != nil {
				return err
			}

			cmd := exec.Command(os.Args[0], "cluster", "proxy", "--kind", *kind, "--sock_path", *sockPath, "--cluster", resolved.ClusterId)
			cmd.SysProcAttr = &syscall.SysProcAttr{
				Foreground: false,
				Setsid:     true,
			}

			if err := cmd.Start(); err != nil {
				return err
			}

			pid := cmd.Process.Pid
			// Make sure the child process is not cleaned up on exit.
			if err := cmd.Process.Release(); err != nil {
				return err
			}

			return os.WriteFile(*background, []byte(fmt.Sprintf("%d", pid)), 0644)
		}

		return runProxy(ctx, *cluster, *kind, *sockPath)
	})

	return cmd
}

func runProxy(ctx context.Context, clusterReq, kind, socketPath string) error {
	if clusterReq == "" || kind == "" {
		return fnerrors.New("--cluster and --kind are required")
	}

	cluster, err := establishCluster(ctx, clusterReq)
	if err != nil {
		return err
	}

	var port int
	switch kind {
	case "buildkit":
		port = int(cluster.BuildCluster.Colocated.TargetPort)

	case "docker":
		port = 2375

	default:
		return fnerrors.New("unrecognized kind %q", kind)
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		_ = api.StartRefreshing(ctx, api.Endpoint, cluster.ClusterId, func(err error) error {
			fmt.Fprintf(console.Warnings(ctx), "Failed to refresh cluster: %v\n", err)
			return nil
		})
	}()

	defer cancel()

	if _, err := runUnixSocketProxy(ctx, cluster.ClusterId, unixSockProxyOpts{
		Kind:       kind,
		SocketPath: socketPath,
		Blocking:   true,
		Connect: func(ctx context.Context) (net.Conn, error) {
			return api.DialPort(ctx, cluster.Cluster, port)
		},
		AnnounceSocket: func(socketPath string) {
			fmt.Fprintf(console.Stdout(ctx), "Listening on %s\n", socketPath)
		},
	}); err != nil {
		return err
	}

	return nil
}

func establishCluster(ctx context.Context, request string) (*api.CreateClusterResult, error) {
	if request == buildCluster || request == buildClusterArm64 {
		response, err := api.EnsureBuildCluster(ctx, api.Endpoint, buildClusterOpts(request))
		if err != nil {
			return nil, err
		}

		if response.BuildCluster == nil || response.BuildCluster.Colocated == nil {
			return nil, fnerrors.New("cluster is not a build cluster")
		}

		if err := waitUntilReady(ctx, response); err != nil {
			return nil, err
		}

		return response, nil
	}

	response, err := api.GetCluster(ctx, api.Endpoint, request)
	if err != nil {
		return nil, err
	}

	return &api.CreateClusterResult{
		ClusterId:    response.Cluster.ClusterId,
		Cluster:      response.Cluster,
		BuildCluster: response.BuildCluster,
		Registry:     response.Registry,
	}, nil
}

func buildClusterOpts(buildCluster string) api.EnsureBuildClusterOpts {
	var opts api.EnsureBuildClusterOpts
	if buildCluster == buildClusterArm64 {
		opts.Features = []string{"EXP_ARM64_CLUSTER"}
	}

	return opts
}
