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
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
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

		response, err := api.EnsureCluster(ctx, api.Endpoint, clusterId)
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

func NewDockerLoginCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docker-login",
		Short:  "Log into the Namespace Cloud private registry for use with Docker.",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		stdout := console.Stdout(ctx)
		response, err := api.GetImageRegistry(ctx, api.Endpoint)
		if err != nil {
			return err
		}

		token, err := fnapi.FetchTenantToken(ctx)
		if err != nil {
			return err
		}

		cfg := config.LoadDefaultConfigFile(console.Stderr(ctx))

		for _, x := range []*api.ImageRegistry{response.Registry, response.NSCR} {
			if x != nil {
				if err := cfg.GetCredentialsStore(response.Registry.EndpointAddress).Store(types.AuthConfig{
					ServerAddress: x.EndpointAddress,
					Username:      "tenant-token",
					Password:      token.Raw(),
				}); err != nil {
					return err
				}
			}
		}

		if *outputRegistryPath != "" {
			// If user wants the registry in output file,
			// give priority to the newer nscr.io registry
			registryEp := response.Registry.EndpointAddress
			if response.NSCR != nil {
				registryEp = fmt.Sprintf("%s/%s", response.NSCR.EndpointAddress, response.NSCR.Repository)
			}
			if err := os.WriteFile(*outputRegistryPath, []byte(registryEp), 0644); err != nil {
				return fnerrors.New("failed to write %q: %w", *outputRegistryPath, err)
			}
		}

		cfgFile := filepath.Join(config.Dir(), config.ConfigFileName)

		info, err := os.Stat(cfgFile)
		if err != nil {
			return fnerrors.New("failed to describe %q: %w", cfgFile, err)
		}

		if err := files.WriteJson(cfgFile, cfg, info.Mode()); err != nil {
			return fnerrors.New("failed to write %q: %w", cfgFile, err)
		}

		if nscr := response.NSCR; nscr != nil {
			fmt.Fprintf(stdout, "\nYou are now logged into your Workspace container registry:\n\n  %s/%s", nscr.EndpointAddress, nscr.Repository)
			fmt.Fprintf(stdout, "\n\nRun your first build with:\n\n  $ nsc build . -t %s/%s/test --push", nscr.EndpointAddress, nscr.Repository)
		}

		fmt.Fprintf(stdout, "\n\nVisit our docs for more details on Remote Builds:\n\n  https://cloud.namespace.so/docs/features/builds\n\n")

		return nil
	})

	return cmd
}
