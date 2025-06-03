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
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/process"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
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
	cluster := cmd.Flags().String("cluster", "", "Cluster ID to proxy.")
	background := cmd.Flags().String("background", "", "If specified runs the proxy in the background, and writes the process PID to the specified path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *cluster == "" || *kind == "" {
			return fnerrors.Newf("--cluster and --kind are required")
		}

		if *background != "" {
			if *sockPath == "" {
				return fnerrors.Newf("--background requires --sock_path")
			}

			// Make sure the cluster exists before going to the background.
			resolved, err := ensureCluster(ctx, *cluster)
			if err != nil {
				return err
			}

			return setupBackgroundProxy(ctx, resolved.ClusterId, *kind, *sockPath, *background)
		}

		return deprecateRunProxy(ctx, *cluster, *kind, *sockPath)
	})

	return cmd
}

func setupBackgroundProxy(ctx context.Context, clusterId, kind, sockPath, pidFile string) error {
	cmd := exec.Command(os.Args[0], "cluster", "proxy", "--kind", kind, "--sock_path", sockPath, "--cluster", clusterId, "--region", endpoint.RegionName)
	process.SetSIDAttr(cmd, true)
	process.ForegroundAttr(cmd, false)

	if err := cmd.Start(); err != nil {
		return err
	}

	pid := cmd.Process.Pid
	// Make sure the child process is not cleaned up on exit.
	if err := cmd.Process.Release(); err != nil {
		return err
	}

	ctx, done := context.WithTimeout(ctx, 5*time.Second)
	defer done()

	// Wait until the socket is up.
	if err := waitForFile(ctx, sockPath); err != nil {
		return fnerrors.Newf("socket didn't come up in time: %v", err)
	}

	if pidFile != "" {
		return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
	}

	return nil
}

func deprecateRunProxy(ctx context.Context, clusterReq, kind, socketPath string) error {
	if clusterReq == "" || kind == "" {
		return fnerrors.Newf("--cluster and --kind are required")
	}

	cluster, err := ensureCluster(ctx, clusterReq)
	if err != nil {
		return err
	}

	var connect func(context.Context) (net.Conn, error)

	if kind == "buildkit" {
		buildkitSvc := api.ClusterService(cluster.Cluster, "buildkit")
		if buildkitSvc == nil || buildkitSvc.Endpoint == "" {
			return fnerrors.Newf("instance is missing buildkit")
		}

		if buildkitSvc.Status != "READY" {
			return fnerrors.Newf("expected buildkit to be READY, saw %q", buildkitSvc.Status)
		}

		connect = func(ctx context.Context) (net.Conn, error) {
			return api.DialEndpoint(ctx, buildkitSvc.Endpoint)
		}
	} else {
		connect = func(ctx context.Context) (net.Conn, error) {
			token, err := fnapi.FetchToken(ctx)
			if err != nil {
				return nil, err
			}

			return connectToSocket(ctx, token, cluster.Cluster, kind)
		}
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		_ = api.StartRefreshing(ctx, api.Methods, cluster.Cluster, func(err error) error {
			fmt.Fprintf(console.Warnings(ctx), "Failed to refresh instance: %v\n", err)
			return nil
		})
	}()

	defer cancel()

	if _, err := runUnixSocketProxy(ctx, cluster.ClusterId, unixSockProxyOpts{
		Kind:       kind,
		SocketPath: socketPath,
		Blocking:   true,
		Connect:    connect,
		AnnounceSocket: func(socketPath string) {
			fmt.Fprintf(console.Stdout(ctx), "Listening on %s\n", socketPath)
		},
	}); err != nil {
		return err
	}

	return nil
}

func ensureCluster(ctx context.Context, clusterID string) (*api.CreateClusterResult, error) {
	response, err := api.EnsureCluster(ctx, api.Methods, nil, clusterID)
	if err != nil {
		return nil, err
	}

	return &api.CreateClusterResult{
		ClusterId: response.Cluster.ClusterId,
		Cluster:   response.Cluster,
	}, nil
}

func waitForFile(ctx context.Context, path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		time.Sleep(500 * time.Millisecond)
	}
}
