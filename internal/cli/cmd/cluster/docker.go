// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

		p, err := runUnixSocketProxy(ctx, clusterId, unixSockProxyOpts{
			Kind: "docker",
			Connect: func(ctx context.Context) (net.Conn, error) {
				return api.DialPort(ctx, response.Cluster, 2375)
			},
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
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		response, err := api.GetImageRegistry(ctx, api.Endpoint)
		if err != nil {
			return err
		}

		if *outputRegistryPath != "" {
			if err := os.WriteFile(*outputRegistryPath, []byte(response.Registry.EndpointAddress), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputRegistryPath, err)
			}
		}

		t, err := api.RegistryCreds(ctx)
		if err != nil {
			return err
		}

		cfg := config.LoadDefaultConfigFile(console.Stderr(ctx))

		cfg.GetCredentialsStore(response.Registry.EndpointAddress).Store(types.AuthConfig{
			Username: t.Username,
			Password: t.Password,
		})

		cfgFile := filepath.Join(config.Dir(), config.ConfigFileName)

		info, err := os.Stat(cfgFile)
		if err != nil {
			return fnerrors.New("failed to describe %q: %w", cfgFile, err)
		}

		if err := files.WriteJson(cfgFile, cfg, info.Mode()); err != nil {
			return fnerrors.New("failed to write %q: %w", cfgFile, err)
		}

		return nil
	})

	return cmd
}
