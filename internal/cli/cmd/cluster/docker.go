// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const (
	dockerUsername   = "tenant-token"
	credHelperBinary = "nsc"
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

		cfg := config.LoadDefaultConfigFile(console.Stderr(ctx))

		if cfg.CredentialHelpers == nil {
			cfg.CredentialHelpers = map[string]string{}
		}

		for _, reg := range []*api.ImageRegistry{response.Registry, response.NSCR} {
			if reg != nil {
				cfg.CredentialHelpers[reg.EndpointAddress] = credHelperBinary
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
			fmt.Fprintf(stdout, "\n\nRun your first build with:\n\n  $ nsc build . --name test:v0.0.1 --push")
		}

		fmt.Fprintf(stdout, "\n\nVisit our docs for more details on Remote Builds:\n\n  https://cloud.namespace.so/docs/features/builds\n\n")

		return nil
	})

	return cmd
}

func NewDockerCredHelperStoreCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "store",
		Short:  "Unimplemented",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return fnerrors.New("unimplemented")
	})

	return cmd
}

func NewDockerCredHelperGetCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Get Workspace's container registry credetial",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		done := console.EnterInputMode(ctx)
		defer done()

		input, err := readStdin()
		if err != nil {
			return fnerrors.New("failed to read from stdin: %w", err)
		}
		regURL := string(input)

		resp, err := api.GetImageRegistry(ctx, api.Endpoint)
		if err != nil {
			return fnerrors.New("failed to get nscloud registries: %w", err)
		}

		for _, reg := range []*api.ImageRegistry{resp.Registry, resp.NSCR} {
			if reg != nil && regURL == reg.EndpointAddress {
				token, err := fnapi.FetchTenantToken(ctx)
				if err != nil {
					return fnerrors.New("failed to fetch tenant token: %w", err)
				}

				c := credHelperGetOutput{
					ServerURL: reg.EndpointAddress,
					Username:  dockerUsername,
					Secret:    token.Raw(),
				}

				buf, err := json.Marshal(c)
				if err != nil {
					return fnerrors.New("failed to marshal output JSON: %w", err)
				}

				fmt.Println(string(buf))
				return nil
			}
		}

		return fnerrors.New("credentials not found in nscloud")

	})

	return cmd
}

func NewDockerCredHelperListCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "list",
		Short:  "List Workspace's container registry credetials",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		done := console.EnterInputMode(ctx)
		defer done()

		resp, err := api.GetImageRegistry(ctx, api.Endpoint)
		if err != nil {
			return fnerrors.New("failed to get nscloud registries: %w", err)
		}

		output := map[string]string{}

		for _, reg := range []*api.ImageRegistry{resp.Registry, resp.NSCR} {
			if reg != nil {
				output[reg.EndpointAddress] = dockerUsername
			}
		}

		buf, err := json.Marshal(output)
		if err != nil {
			return fnerrors.New("failed to marshal output JSON: %w", err)
		}

		fmt.Println(string(buf))
		return nil
	})

	return cmd
}

func NewDockerCredHelperEraseCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "erase",
		Short:  "Unimplemented",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return fnerrors.New("unimplemented")
	})

	return cmd
}

type credHelperGetOutput struct {
	ServerURL string
	Username  string
	Secret    string
}

func readStdin() ([]byte, error) {
	reader := bufio.NewReader(os.Stdin)
	s, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return nil, err
	}
	return bytes.TrimSpace([]byte(s)), nil
}
