// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	moby_buildkit_v1 "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
)

var (
	BuildkitSecrets string
	ForwardKeychain = false
)

const SSHAgentProviderID = "default"

type GatewayClient struct {
	*client.Client

	buildkitInDocker bool
	clientOpts       builtkitOpts
}

type builtkitOpts struct {
	SupportsOCILayoutInput  bool
	SupportsOCILayoutExport bool
	SupportsCanonicalBuilds bool
	HostPlatform            specs.Platform
}

func (cli *GatewayClient) UsesDocker() bool                                     { return cli.buildkitInDocker }
func (cli *GatewayClient) BuildkitOpts() builtkitOpts                           { return cli.clientOpts }
func (cli *GatewayClient) MakeClient(_ context.Context) (*GatewayClient, error) { return cli, nil }

type clientInstance struct {
	conf      cfg.Configuration
	overrides *Overrides

	compute.DoScoped[*GatewayClient] // Only connect once per configuration.
}

var OverridesConfigType = cfg.DefineConfigType[*Overrides]()

func Client(ctx context.Context, config cfg.Configuration, targetPlatform *specs.Platform) (*GatewayClient, error) {
	return compute.GetValue(ctx, MakeClient(config, targetPlatform))
}

func DeferClient(config cfg.Configuration, targetPlatform *specs.Platform) ClientFactory {
	return deferredMakeClient{config, targetPlatform}
}

type deferredMakeClient struct {
	config         cfg.Configuration
	targetPlatform *specs.Platform
}

func (d deferredMakeClient) MakeClient(ctx context.Context) (*GatewayClient, error) {
	return Client(ctx, d.config, d.targetPlatform)
}

func MakeClient(config cfg.Configuration, targetPlatform *specs.Platform) compute.Computable[*GatewayClient] {
	var conf *Overrides

	if targetPlatform != nil {
		conf, _ = OverridesConfigType.CheckGetForPlatform(config, *targetPlatform)
	} else {
		conf, _ = OverridesConfigType.CheckGet(config)
	}

	if conf.BuildkitAddr == "" && conf.HostedBuildCluster == nil && conf.ContainerName == "" {
		conf.ContainerName = DefaultContainerName
	}

	return &clientInstance{conf: config, overrides: conf}
}

var _ compute.Computable[*GatewayClient] = &clientInstance{}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Proto("conf", c.overrides)
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (*GatewayClient, error) {
	if c.overrides.BuildkitAddr != "" {
		cli, err := client.New(ctx, c.overrides.BuildkitAddr)
		if err != nil {
			return nil, err
		}

		return newClient(ctx, cli, false)
	}

	if c.overrides.HostedBuildCluster != nil {
		fmt.Fprintf(console.Debug(ctx), "buildkit: connecting to nscloud %s/%d\n",
			c.overrides.HostedBuildCluster.ClusterId, c.overrides.HostedBuildCluster.TargetPort)

		cluster, err := api.GetCluster(ctx, api.Endpoint, c.overrides.HostedBuildCluster.ClusterId)
		if err != nil {
			return nil, fnerrors.InternalError("failed to connect to buildkit in cluster: %w", err)
		}

		return waitAndConnect(ctx, func() (*client.Client, error) {
			return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return api.DialPort(ctx, cluster.Cluster, int(c.overrides.HostedBuildCluster.TargetPort))
			}))
		})
	}

	if c.overrides.ColocatedBuildCluster != nil {
		conf := c.overrides.ColocatedBuildCluster
		fmt.Fprintf(console.Debug(ctx), "buildkit: connecting to co-located %v (port %d)\n", conf.MatchingPodLabels, conf.TargetPort)

		k, err := kubernetes.ConnectToCluster(ctx, c.conf)
		if err != nil {
			return nil, fnerrors.InternalError("failed to connect to co-located build cluster: %w", err)
		}

		return waitAndConnect(ctx, func() (*client.Client, error) {
			return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return k.RawDialServer(ctx, conf.Namespace, conf.MatchingPodLabels, &schema.Endpoint_Port{ContainerPort: conf.TargetPort})
			}))
		})
	}

	localAddr, err := EnsureBuildkitd(ctx, c.overrides.ContainerName)
	if err != nil {
		return nil, err
	}

	cli, err := client.New(ctx, localAddr)
	if err != nil {
		return nil, err
	}

	// When disconnecting often get:
	//
	// WARN[0012] commandConn.CloseWrite: commandconn: failed to wait: signal: terminated
	//
	// compute.On(ctx).Cleanup(tasks.Action("buildkit.disconnect"), func(ctx context.Context) error {
	// 	return cli.Close()
	// })

	return newClient(ctx, cli, true)
}

func waitAndConnect(ctx context.Context, connect func() (*client.Client, error)) (*GatewayClient, error) {
	if err := waitForBuildkit(ctx, connect); err != nil {
		return nil, err
	}

	cli, err := connect()
	if err != nil {
		return nil, err
	}

	return newClient(ctx, cli, false)
}

func newClient(ctx context.Context, cli *client.Client, docker bool) (*GatewayClient, error) {
	var opts builtkitOpts

	workers, err := cli.ControlClient().ListWorkers(ctx, &moby_buildkit_v1.ListWorkersRequest{})
	if err != nil {
		return nil, fnerrors.InvocationError("buildkit", "failed to retrieve worker list: %w", err)
	}

	var hostPlatform *specs.Platform
	for _, x := range workers.Record {
		// We assume here that by convention the first platform is the host platform.
		if len(x.Platforms) > 0 {
			hostPlatform = &specs.Platform{
				Architecture: x.Platforms[0].Architecture,
				OS:           x.Platforms[0].OS,
			}
			break
		}
	}

	if hostPlatform == nil {
		return nil, fnerrors.InvocationError("buildkit", "no worker with platforms declared?")
	}

	opts.HostPlatform = *hostPlatform

	response, err := cli.ControlClient().Info(ctx, &moby_buildkit_v1.InfoRequest{})
	if err == nil {
		x, _ := json.MarshalIndent(response.GetBuildkitVersion(), "", "  ")
		fmt.Fprintf(console.Debug(ctx), "buildkit: version: %v\n", string(x))

		if semver.Compare(response.BuildkitVersion.GetVersion(), "v0.11.0-rc1") >= 0 {
			opts.SupportsOCILayoutInput = true
			opts.SupportsCanonicalBuilds = true
			opts.SupportsOCILayoutExport = false // Some casual testing seems to indicate that this is actually slower.
		}
	} else {
		fmt.Fprintf(console.Debug(ctx), "buildkit: Info failed: %v\n", err)
	}

	return &GatewayClient{Client: cli, buildkitInDocker: docker, clientOpts: opts}, nil
}
