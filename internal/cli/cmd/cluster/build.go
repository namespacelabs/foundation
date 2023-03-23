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
	"strings"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/moby/buildkit/client"
	"github.com/spf13/cobra"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/sdk/buildctl"
	"namespacelabs.dev/foundation/internal/sdk/host"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewBuildctlCmd() *cobra.Command {
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

	fmt.Fprintf(console.Debug(ctx), "buildctl %s\n", strings.Join(cmdLine, " "))

	buildctl := exec.CommandContext(ctx, string(buildctlBin), cmdLine...)
	buildctl.Env = os.Environ()
	buildctl.Env = append(buildctl.Env, fmt.Sprintf("DOCKER_CONFIG="+p.DockerConfigDir))

	return localexec.RunInteractive(ctx, buildctl)
}

type buildProxy struct {
	BuildkitAddr     string
	DockerConfigDir  string
	RegistryEndpoint string
	Repository       string
	Cleanup          func()
}

func runBuildProxy(ctx context.Context) (*buildProxy, error) {
	existing := config.LoadDefaultConfigFile(console.Stderr(ctx))

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

	p, err := runUnixSocketProxy(ctx, response.ClusterId, unixSockProxyOpts{
		Kind: "buildkit",
		Connect: func(ctx context.Context) (net.Conn, error) {
			return connect(ctx, response)
		},
	})
	if err != nil {
		return nil, err
	}

	t, err := api.RegistryCreds(ctx)
	if err != nil {
		p.Cleanup()
		return nil, err
	}

	// We don't copy over all authentication settings; only some.
	// XXX replace with custom buildctl invocation that merges auth in-memory.
	newConfig := configfile.ConfigFile{
		AuthConfigs:       existing.AuthConfigs,
		CredentialHelpers: existing.CredentialHelpers,
		CredentialsStore:  existing.CredentialsStore,
	}

	newConfig.AuthConfigs[response.Registry.EndpointAddress] = types.AuthConfig{
		Username: t.Username,
		Password: t.Password,
	}

	credsFile := filepath.Join(p.TempDir, config.ConfigFileName)
	if err := files.WriteJson(credsFile, newConfig, 0600); err != nil {
		p.Cleanup()
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		_ = api.StartRefreshing(ctx, api.Endpoint, response.ClusterId, func(err error) error {
			fmt.Fprintf(console.Warnings(ctx), "Failed to refresh cluster: %v\n", err)
			return nil
		})
	}()

	return &buildProxy{p.SocketAddr, p.TempDir, response.Registry.EndpointAddress, response.Registry.Repository, func() {
		cancel()
		p.Cleanup()
	}}, nil
}

func waitUntilReady(ctx context.Context, response *api.CreateClusterResult) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return buildkitfw.WaitReadiness(ctx, func() (*client.Client, error) {
			// We must fetch a token with our parent context, so we get a task sink etc.
			token, err := fnapi.FetchTenantToken(ctx)
			if err != nil {
				return nil, err
			}

			return client.New(ctx, response.ClusterId, client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialPortWithToken(ctx, token, response.Cluster, int(response.BuildCluster.Colocated.TargetPort))
			}))
		})
	})
}

func serveBuildProxy(ctx context.Context, listener net.Listener, response *api.CreateClusterResult) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			defer conn.Close()

			peerConn, err := connect(ctx, response)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
				return
			}

			defer peerConn.Close()

			go func() {
				_, _ = io.Copy(conn, peerConn)
			}()

			_, _ = io.Copy(peerConn, conn)
		}()
	}
}

func connect(ctx context.Context, response *api.CreateClusterResult) (net.Conn, error) {
	return api.DialPort(ctx, response.Cluster, int(response.BuildCluster.Colocated.TargetPort))
}

func NewBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build an image in a build cluster.",
		Args:  cobra.MaximumNArgs(1),
	}

	dockerFile := cmd.Flags().StringP("file", "f", "", "If set, specifies what Dockerfile to build.")
	push := cmd.Flags().Bool("push", false, "If specified, pushes the image to the target repository.")
	tags := cmd.Flags().StringSliceP("tag", "t", nil, "Attach a tags to the image.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, specifiedArgs []string) error {

		buildctlBin, err := buildctl.EnsureSDK(ctx, host.HostPlatform())
		if err != nil {
			return fnerrors.New("failed to download buildctl: %w", err)
		}

		p, err := runBuildProxy(ctx)
		if err != nil {
			return err
		}

		defer p.Cleanup()

		contextDir := "."
		if len(specifiedArgs) > 0 {
			contextDir = specifiedArgs[0]
		}

		args := []string{
			"build",
			"--frontend=dockerfile.v0",
			"--local", "context=" + contextDir,
			"--local", "dockerfile=" + contextDir,
		}

		if *dockerFile != "" {
			args = append(args, "--opt", "filename="+*dockerFile)
		}

		var complete func() error

		if *push {
			imageNames := []string{}
			for _, tag := range *tags {
				parsed, err := name.NewTag(tag)
				if err != nil {
					return fmt.Errorf("invalid tag %s: %w", tag, err)
				}

				imageNames = append(imageNames, parsed.Name())
			}

			args = append(args,
				// buildctl parses output as csv; need to quote to pass commas to `name`.
				"--output", fmt.Sprintf("type=image,push=true,%q", "name="+strings.Join(imageNames, ",")),
			)

			complete = func() error {
				fmt.Fprintf(console.Stdout(ctx), "Pushed:\n")
				for _, imageName := range imageNames {
					fmt.Fprintf(console.Stdout(ctx), "  %s\n", imageName)
				}

				return nil
			}
		} else {
			f, err := os.CreateTemp("", "docker-image-nsc")
			if err != nil {
				return err
			}

			defer os.Remove(f.Name())

			// We don't actually need it.
			f.Close()

			imageNames := []string{}
			for _, tag := range *tags {
				parsed, err := name.NewTag(tag)
				if err != nil {
					return fmt.Errorf("invalid tag %s: %w", tag, err)
				}
				imageNames = append(imageNames, parsed.Name())
			}

			if len(imageNames) > 0 {
				// buildctl parses output as csv; need to quote to pass commas to `name`.
				args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s,%q",
					f.Name(), "name="+strings.Join(imageNames, ",")))
			} else {
				args = append(args, "--output", fmt.Sprintf("type=docker,dest=%s",
					f.Name()))
			}

			complete = func() error {
				t := time.Now()
				dockerLoad := exec.CommandContext(ctx, "docker", "load", "-i", f.Name())
				if err := localexec.RunInteractive(ctx, dockerLoad); err != nil {
					return err
				}
				took := time.Since(t)
				fmt.Fprintf(console.Stdout(ctx), "Took %v to upload the image to docker.\n", took)
				return nil
			}
		}

		if err := runBuildctl(ctx, buildctlBin, p, args...); err != nil {
			return err
		}

		return complete()
	})

	return cmd
}
