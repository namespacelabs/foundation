// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/process"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/providers/nscloud/endpoint"
)

const (
	buildCluster      = "build-cluster"
	buildClusterArm64 = "build-cluster-arm64"
)

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
