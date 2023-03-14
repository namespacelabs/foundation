// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "proxy",
		Short:  "Runs a unix socket proxy for a well known service.",
		Hidden: true,
	}

	kind := cmd.Flags().String("kind", "", "The service being proxied.")
	sockPath := cmd.Flags().String("sock_path", "", "If specified listens on the specified path.")
	cluster := cmd.Flags().String("cluster", "", "Which cluster to proxy; or 'build-cluster' to proxy the build cluster.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *cluster == "" || *kind == "" {
			return fnerrors.New("--cluster and --kind are required")
		}

		cluster, err := establishCluster(ctx, *cluster)
		if err != nil {
			return err
		}

		var port int
		switch *kind {
		case "buildkit":
			port = int(cluster.BuildCluster.Colocated.TargetPort)

		case "docker":
			port = 2375

		default:
			return fnerrors.New("unrecognized kind %q", *kind)
		}

		if _, err := runUnixSocketProxy(ctx, cluster.ClusterId, unixSockProxyOpts{
			Kind:       *kind,
			SocketPath: *sockPath,
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
	})

	return cmd
}

func establishCluster(ctx context.Context, request string) (*api.CreateClusterResult, error) {
	if request == "build-cluster" {
		response, err := api.EnsureBuildCluster(ctx, api.Endpoint)
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

	return &api.CreateClusterResult{ClusterId: response.Cluster.ClusterId, Cluster: response.Cluster}, nil
}
