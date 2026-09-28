// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

const (
	credHelperBinary = "docker-credential-nsc"
	nscBinary        = "nsc"
)

func NewDockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Docker-related functionality.",
	}

	attach := newDockerAttachCmd()
	fncobra.MarkAsNotSupportedOnWindows(attach)
	cmd.AddCommand(attach) // nsc docker attach-context

	remote := newDockerRemoteCmd()
	fncobra.MarkAsNotSupportedOnWindows(remote)
	cmd.AddCommand(remote)                   // nsc docker remote
	cmd.AddCommand(newDockerLoginCmd(false)) // nsc docker login

	buildx := &cobra.Command{Use: "buildx", Short: "Docker Buildx related functionality."}
	fncobra.MarkAsNotSupportedOnWindows(buildx)
	buildx.AddCommand(newSetupBuildxCmd())
	buildx.AddCommand(newCleanupBuildxCommand())
	buildx.AddCommand(newWireBuildxCommand(true))
	buildx.AddCommand(newStatusBuildxCommand())
	buildx.AddCommand(newDumpListWorkersBuildxCommand())
	buildx.AddCommand(newWaitBuilderCommand())

	cmd.AddCommand(buildx)

	return cmd
}

func newDockerRemoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                "remote ...",
		Short:              "Run docker on the target instance.",
		Args:               cobra.MinimumNArgs(1),
		DisableFlagParsing: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return runPassthrough(ctx, args[0], args[1:])
	})

	return cmd
}

func newDockerPassthroughCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "docker -- ...",
		Short:  "Run docker on the target instance.",
		Args:   cobra.MinimumNArgs(1),
		Hidden: true,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		return runPassthrough(ctx, args[0], args[1:])
	})

	return cmd
}

func runPassthrough(ctx context.Context, clusterId string, args []string) error {
	if len(args) == 0 {
		return fnerrors.Newf("at least one argument to pass to `docker` is required")
	}

	return withDocker(ctx, clusterId, func(ctx context.Context, socketPath string) error {
		return runDocker(ctx, socketPath, args...)
	})
}

func withDocker(ctx context.Context, clusterId string, callback func(context.Context, string) error) error {
	response, err := api.EnsureCluster(ctx, api.Methods, nil, clusterId)
	if err != nil {
		return err
	}

	p, err := runUnixSocketProxy(ctx, clusterId, unixSockProxyOpts{
		Kind: "docker",
		Connect: func(ctx context.Context) (net.Conn, error) {
			token, err := fnapi.FetchToken(ctx)
			if err != nil {
				return nil, err
			}
			return connectToDocker(ctx, token, response.Cluster)
		},
	})
	if err != nil {
		return err
	}

	defer p.Cleanup()

	return callback(ctx, p.SocketAddr)
}

func connectToSocket(ctx context.Context, token fnapi.Token, cluster *api.KubernetesCluster, name string) (net.Conn, error) {
	vars := url.Values{}
	vars.Set("name", fmt.Sprintf("%s-socket", name))
	return api.DialHostedServiceWithToken(ctx, token, cluster, "unixsocket", vars)
}

func connectToDocker(ctx context.Context, token fnapi.Token, cluster *api.KubernetesCluster) (net.Conn, error) {
	return connectToSocket(ctx, token, cluster, "docker")
}

func runDocker(ctx context.Context, socketPath string, args ...string) error {
	cmdLine := []string{"-H", "unix://" + socketPath}
	cmdLine = append(cmdLine, args...)

	docker := exec.CommandContext(ctx, "docker", cmdLine...)
	return localexec.RunInteractive(ctx, docker)
}

func newDockerLoginCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "login",
		Short:  "Log into the Namespace private registry for use with Docker.",
		Args:   cobra.NoArgs,
		Hidden: hidden,
	}

	outputRegistryPath := cmd.Flags().String("output_registry_to", "", "If specified, write the registry address to this path.")
	useCredentialHelper := cmd.Flags().Bool("use_credential_helper", true, "Use nsc's credential helper instead of embedding the credentials.")
	tokenDuration := fncobra.Duration(cmd.Flags(), "token_duration", 8*time.Hour, "The duration of the embedded token (requires --use_credential_helper=false).")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		if *useCredentialHelper && cmd.Flags().Changed("token_duration") {
			return fnerrors.Newf("--token_duration requires --use_credential_helper=false")
		}

		stdout := console.Stdout(ctx)

		response, err := api.GetImageRegistry(ctx, api.Methods)
		if err != nil {
			return err
		}

		cfg := config.LoadDefaultConfigFile(console.Stderr(ctx))

		if cfg.CredentialHelpers == nil {
			cfg.CredentialHelpers = map[string]string{}
		}

		if cfg.AuthConfigs == nil {
			cfg.AuthConfigs = map[string]types.AuthConfig{}
		}

		registries := append(response.ExtraRegistry, []*api.ImageRegistry{response.NSCR}...)
		for _, reg := range registries {
			if reg != nil {
				if *useCredentialHelper {
					cfg.CredentialHelpers[reg.EndpointAddress] = nscBinary

					delete(cfg.AuthConfigs, reg.EndpointAddress)
				} else {
					token, err := fnapi.IssueToken(ctx, *tokenDuration)
					if err != nil {
						return err
					}

					cfg.AuthConfigs[reg.EndpointAddress] = types.AuthConfig{
						ServerAddress: reg.EndpointAddress,
						Username:      nscrRegistryUsername,
						Password:      token,
					}

					delete(cfg.CredentialHelpers, reg.EndpointAddress)
				}
			}
		}

		if *outputRegistryPath != "" {
			if response.NSCR == nil {
				return fnerrors.BadDataError("missing nscr endpoint")
			}

			registryEp := fmt.Sprintf("%s/%s", response.NSCR.EndpointAddress, response.NSCR.Repository)
			if err := os.WriteFile(*outputRegistryPath, []byte(registryEp), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputRegistryPath, err)
			}
		}
		if err := cfg.Save(); err != nil {
			return fnerrors.Newf("failed to save config: %w", err)
		}
		fmt.Fprintf(stdout, "Added Namespace credentials to %s.\n", cfg.Filename)

		if nscr := response.NSCR; nscr != nil {
			fmt.Fprintf(stdout, "\nYou are now logged into your Workspace container registry:\n\n  %s/%s", nscr.EndpointAddress, nscr.Repository)
			fmt.Fprintf(stdout, "\n\nRun your first build with:\n\n  $ nsc build . --name test:v0.0.1 --push")
		}

		fmt.Fprintf(stdout, "\n\nVisit our docs for more details on Remote Builds:\n\n  https://namespace.so/docs/solutions/docker-builders\n\n")

		if _, err := exec.LookPath(credHelperBinary); err != nil {
			style := colors.Ctx(ctx)
			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(stdout)
				fmt.Fprint(stdout, style.Highlight.Apply(fmt.Sprintf("We didn't find %s in your $PATH.", credHelperBinary)))
				fmt.Fprintf(stdout, "\nIt's usually installed along-side nsc; so if you have added nsc to the $PATH, %s will also be available.\n", credHelperBinary)
				fmt.Fprintf(stdout, "\nWhile your $PATH is not updated, accessing nscr.io images from docker-based tools won't work.\nBut you can always use nsc build (as per above) or nsc run.\n")
			} else {
				return fnerrors.Newf("failed to look up nsc in $PATH: %w", err)
			}
		}

		return nil
	})

	return cmd
}
