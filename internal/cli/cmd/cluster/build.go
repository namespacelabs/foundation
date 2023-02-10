// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func newBuildctlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buildctl -- ...",
		Short: "Run buildctl on the target build cluster.",
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		p, err := runBuildProxy(ctx)
		if err != nil {
			return err
		}

		defer p.Cleanup()

		return runBuildctl(ctx, buildctlBin, p, args...)
	})

	return cmd
}

func runBuildctl(ctx context.Context, buildctlBin buildctl.Buildctl, p *buildProxy, args ...string) error {
	cmdLine := []string{"--addr", "unix://" + p.BuildkitAddr}
	cmdLine = append(cmdLine, args...)

	buildctl := exec.CommandContext(ctx, string(buildctlBin), cmdLine...)
	buildctl.Env = append(buildctl.Env, fmt.Sprintf("DOCKER_CONFIG="+p.DockerConfigDir))

	return localexec.RunInteractive(ctx, buildctl)
}

type buildProxy struct {
	BuildkitAddr    string
	DockerConfigDir string
	Cleanup         func()
}

func runBuildProxy(ctx context.Context) (*buildProxy, error) {
	response, err := api.EnsureBuildCluster(ctx, api.Endpoint)
	if err != nil {
		return nil, err
	}

	if response.BuildCluster == nil || response.BuildCluster.Colocated == nil {
		return nil, fnerrors.New("cluster is not a build cluster")
	}

	sockDir, err := dirs.CreateUserTempDir("buildkit", response.Cluster.ClusterId)
	if err != nil {
		return nil, err
	}

	t, err := api.RegistryCreds(ctx)
	if err != nil {
		os.RemoveAll(sockDir)
		return nil, err
	}

	var cfg configfile.ConfigFile
	cfg.AuthConfigs = map[string]types.AuthConfig{
		response.Registry.EndpointAddress: {
			Username: t.Username,
			Password: t.Password,
		},
	}

	credsFile := filepath.Join(sockDir, config.ConfigFileName)
	if err := files.WriteJson(credsFile, cfg, 0600); err != nil {
		os.RemoveAll(sockDir)
		return nil, err
	}

	sockFile := filepath.Join(sockDir, "buildkit.sock")
	listener, err := net.Listen("unix", sockFile)
	if err != nil {
		os.RemoveAll(sockDir)
		return nil, err
	}

	go serveBuildProxy(ctx, listener, response)

	return &buildProxy{sockFile, sockDir, func() { _ = os.RemoveAll(sockDir) }}, nil
}

func serveBuildProxy(ctx context.Context, listener net.Listener, response *api.CreateClusterResult) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			defer conn.Close()

			peerConn, err := api.DialPort(ctx, response.Cluster, int(response.BuildCluster.Colocated.TargetPort))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			go func() {
				_, _ = io.Copy(conn, peerConn)
			}()

			_, _ = io.Copy(peerConn, conn)
		}()
	}
}
