// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/containerd/containerd/platforms"
	moby_buildkit_v1 "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	buildkit "github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/mod/semver"
	buildkitfw "namespacelabs.dev/foundation/framework/build/buildkit"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/cfg/knobs"
	"namespacelabs.dev/foundation/std/tasks"

	_ "github.com/moby/buildkit/client/connhelper/dockercontainer"
)

var (
	BuildkitSecrets string
	ForwardKeychain = true
)

var (
	BuildOnNamespaceCloud           = knobs.Bool("build_in_nscloud", "If set to true, builds are triggered remotely.", false)
	BuildOnNamespaceCloudUnlessHost = knobs.Bool("build_in_nscloud_unless_host", "If set to true, builds that match the host platform run locally. All other builds are triggered remotely.", false)
	BuildOnExistingBuildkit         = knobs.String("buildkit_addr", "The address of an existing buildkitd to use.", "")
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
	platform  *specs.Platform

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

	return &clientInstance{conf: config, overrides: conf, platform: targetPlatform}
}

var _ compute.Computable[*GatewayClient] = &clientInstance{}

func (c *clientInstance) Action() *tasks.ActionEvent {
	return tasks.Action("buildkit.connect")
}

func (c *clientInstance) Inputs() *compute.In {
	return compute.Inputs().Proto("conf", c.overrides).JSON("platform", c.platform)
}

func (c *clientInstance) Compute(ctx context.Context, _ compute.Resolved) (*GatewayClient, error) {
	if addr := BuildOnExistingBuildkit.Get(c.conf); addr != "" {
		fmt.Fprintf(console.Debug(ctx), "buildkit: using existing buildkit: %q\n", addr)

		cli, err := client.New(ctx, addr)
		if err != nil {
			return nil, err
		}

		return newClient(ctx, cli, false)
	}

	if c.overrides.BuildkitAddr != "" {
		cli, err := client.New(ctx, c.overrides.BuildkitAddr)
		if err != nil {
			return nil, err
		}

		return newClient(ctx, cli, false)
	}

	if x := c.overrides.HostedBuildCluster; x != nil {
		fmt.Fprintf(console.Debug(ctx), "buildkit: connecting to nscloud %s (port: %d endpoint: %s)\n", x.ClusterId, x.TargetPort, x.Endpoint)

		if x.Endpoint == "" {
			cluster, err := api.EnsureCluster(ctx, api.Methods, nil, x.ClusterId)
			if err != nil {
				return nil, fnerrors.InternalError("failed to connect to buildkit in cluster: %w", err)
			}

			return useRemoteCluster(ctx, cluster.Cluster, int(x.TargetPort))
		}

		return useRemoteClusterViaEndpoint(ctx, x.Endpoint)
	}

	if c.overrides.ColocatedBuildCluster != nil {
		conf := c.overrides.ColocatedBuildCluster
		fmt.Fprintf(console.Debug(ctx), "buildkit: connecting to co-located %v (port %d)\n", conf.MatchingPodLabels, conf.TargetPort)

		k, err := kubernetes.ConnectToCluster(ctx, c.conf)
		if err != nil {
			return nil, fnerrors.InternalError("failed to connect to co-located build cluster: %w", err)
		}

		return waitAndConnect(ctx, func(ctx context.Context) (*client.Client, error) {
			return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return k.RawDialServer(ctx, conf.Namespace, conf.MatchingPodLabels, &schema.Endpoint_Port{ContainerPort: conf.TargetPort})
			}))
		})
	}

	if buildRemotely(c.conf, c.platform) {
		parsedPlatform, err := parsePlatformOrDefault(c.platform)
		if err != nil {
			return nil, fnerrors.InternalError("failed to parse build platform: %v", err)
		}

		stateDir, err := dirs.Ensure(cluster.DetermineStateDir("", cluster.BuildkitProxyPath))
		if err != nil {
			return nil, fnerrors.InternalError("failed to ensure state dir: %v", err)
		}

		builderConfigs, err := cluster.PrepareServerSideBuildxProxy(ctx, stateDir, []api.BuildPlatform{parsedPlatform}, api.BuilderConfiguration{})
		if err != nil {
			return nil, err
		}
		if len(builderConfigs) != 1 {
			return nil, fnerrors.InternalError("expected one builder config, got %d", len(builderConfigs))
		}

		endpoint := builderConfigs[0].FullBuildkitEndpoint
		fmt.Fprintf(console.Info(ctx), "buildkit: using server-side build proxy (endpoint: %s)\n", endpoint)

		isServerSideProxy, err := cluster.TestServerSideBuildxProxyConnectivity(ctx, builderConfigs[0])
		if err != nil {
			fmt.Fprintf(console.Warnings(ctx), "buildkit: connectivity check to '%s' failed: %v\n", endpoint, err)
		}

		if !isServerSideProxy {
			fmt.Fprintf(console.Warnings(ctx), "buildkit: '%s' has connectivity but doesn't seem to be Namespace Build Ingress\n", endpoint)
		}

		return useRemoteClusterViaMtls(ctx, builderConfigs[0])
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

func buildRemotely(conf cfg.Configuration, platform *specs.Platform) bool {
	if BuildOnNamespaceCloud.Get(conf) {
		return true
	}

	target := formatPlatformOrDefault(platform)
	host := platforms.Format(docker.HostPlatform())

	if BuildOnNamespaceCloudUnlessHost.Get(conf) && target != host {
		return true
	}

	return false
}

func formatPlatformOrDefault(p *specs.Platform) string {
	if p != nil {
		return platforms.Format(*p)
	}

	return platforms.Format(docker.HostPlatform())
}

func parsePlatformOrDefault(p *specs.Platform) (api.BuildPlatform, error) {
	if p != nil {
		return api.ParseBuildPlatform(p.Architecture)
	}

	return api.ParseBuildPlatform(docker.HostPlatform().Architecture)
}

func useRemoteCluster(ctx context.Context, cluster *api.KubernetesCluster, port int) (*GatewayClient, error) {
	// We must fetch a token with our parent context, so we get a task sink etc.
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return waitAndConnect(ctx, func(ctx context.Context) (*client.Client, error) {
		return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			// Do the expirations work? We don't re-fetch tokens here yet.
			return api.DialPortWithToken(ctx, token, cluster, port)
		}))
	})
}

func useRemoteClusterViaEndpoint(ctx context.Context, endpoint string) (*GatewayClient, error) {
	// We must fetch a token with our parent context, so we get a task sink etc.
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}

	return waitAndConnect(ctx, func(ctx context.Context) (*client.Client, error) {
		return client.New(ctx, "buildkitd", client.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			// Do the expirations work? We don't re-fetch tokens here yet.
			return api.DialEndpointWithToken(ctx, token, endpoint)
		}))
	})
}

func useRemoteClusterViaMtls(ctx context.Context, bc cluster.BuilderConfig) (*GatewayClient, error) {
	return waitAndConnect(ctx, func(ctx context.Context) (*client.Client, error) {
		return client.New(ctx, bc.FullBuildkitEndpoint, client.WithCredentials(bc.ClientCertPath, bc.ClientKeyPath), client.WithServerConfig("", bc.ServerCAPath))

	})
}

func useBuildClusterCluster(ctx context.Context, bp cluster.BuildCluster) (*GatewayClient, error) {
	sink := tasks.SinkFrom(ctx)

	return waitAndConnect(ctx, func(innerCtx context.Context) (*client.Client, error) {
		return client.New(innerCtx, "buildkitd", client.WithContextDialer(func(innerCtx context.Context, _ string) (net.Conn, error) {
			conn, _, err := bp.NewConn(tasks.WithSink(innerCtx, sink))
			return conn, err
		}))
	})
}

func waitReadiness(ctx context.Context, connect func(ctx context.Context) (*buildkit.Client, error)) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return buildkitfw.WaitReadiness(ctx, 5*time.Second, connect)
	})
}

func waitAndConnect(ctx context.Context, connect func(ctx context.Context) (*client.Client, error)) (*GatewayClient, error) {
	if err := waitReadiness(ctx, connect); err != nil {
		return nil, err
	}

	cli, err := connect(ctx)
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
