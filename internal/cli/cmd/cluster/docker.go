// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
	"os/exec"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func newDockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docker -- ...",
		Short:  "Run docker on the target cluster.",
		Args:   cobra.MinimumNArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		clusterId := args[0]
		args = args[1:]

		response, err := api.GetCluster(ctx, api.Endpoint, clusterId)
		if err != nil {
			return err
		}

		p, err := runUnixSocketProxy(ctx, "docker", clusterId, func(ctx context.Context) (net.Conn, error) {
			return api.DialPort(ctx, response.Cluster, 2375)
		})
		if err != nil {
			return err
		}

		defer p.Cleanup()

		return runDocker(ctx, p, args...)
	})

	return cmd
}

func runDocker(ctx context.Context, p *unixSockProxy, args ...string) error {
	cmdLine := []string{"-H", "unix://" + p.SocketAddr}
	cmdLine = append(cmdLine, args...)

	docker := exec.CommandContext(ctx, "docker", cmdLine...)
	return localexec.RunInteractive(ctx, docker)
}

func newDockerLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docker-login",
		Short:  "Log into the Namespace Cloud private registry for use with Docker.",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		response, err := api.GetCluster(ctx, api.Endpoint, args[0])
		if err != nil {
			return err
		}

		token, err := auth.LoadTenantToken(ctx)
		if err != nil {
			return err
		}

		cmdLine := []string{"login", response.Registry.EndpointAddress, "-u", "tenant-token", "-p", token.TenantToken}
		docker := exec.CommandContext(ctx, "docker", cmdLine...)
		return localexec.RunInteractive(ctx, docker)
	})

	return cmd
}
