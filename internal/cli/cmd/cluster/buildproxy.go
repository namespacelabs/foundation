// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	controlapi "github.com/moby/buildkit/api/services/control"
	"golang.org/x/sys/unix"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/files"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	nscrRegistryUsername = "token"
)

type BuildClusterInstance struct {
	platform api.BuildPlatform

	mu            sync.Mutex
	previous      *api.CreateClusterResult
	cancelRefresh func()
}

func (bp *BuildClusterInstance) NewConn(parentCtx context.Context) (net.Conn, error) {
	// Wait at most 20 seconds to create a connection to a build cluster.
	ctx, done := context.WithTimeout(parentCtx, 20*time.Second)
	defer done()

	// This is not our usual play; we're doing a lot of work with the lock held.
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.previous != nil && (bp.previous.BuildCluster == nil || bp.previous.BuildCluster.Resumable) {
		if _, err := api.EnsureCluster(ctx, api.Methods, bp.previous.ClusterId); err == nil {
			return bp.rawDial(ctx, bp.previous)
		}
	}

	response, err := api.CreateBuildCluster(ctx, api.Methods, bp.platform)
	if err != nil {
		return nil, err
	}

	if bp.cancelRefresh != nil {
		bp.cancelRefresh()
		bp.cancelRefresh = nil
	}

	if bp.previous == nil || bp.previous.ClusterId != response.ClusterId {
		if err := waitUntilReady(ctx, response); err != nil {
			return nil, fmt.Errorf("failed to wait for buildkit to become ready: %w", err)
		}
	}

	if response.BuildCluster != nil && !response.BuildCluster.DoesNotRequireRefresh {
		bp.cancelRefresh = api.StartBackgroundRefreshing(parentCtx, response.ClusterId)
	}

	bp.previous = response

	return bp.rawDial(ctx, response)
}

func (bp *BuildClusterInstance) Cleanup() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.cancelRefresh != nil {
		bp.cancelRefresh()
		bp.cancelRefresh = nil
	}

	return nil
}

func (bp *BuildClusterInstance) rawDial(ctx context.Context, response *api.CreateClusterResult) (net.Conn, error) {
	buildkitSvc, err := resolveBuildkitService(response)
	if err != nil {
		return nil, err
	}

	return api.DialEndpoint(ctx, buildkitSvc.Endpoint)
}

func NewBuildClusterInstance(ctx context.Context, platformStr string) (*BuildClusterInstance, error) {
	clusterProfiles, err := api.GetProfile(ctx, api.Methods)
	if err != nil {
		return nil, err
	}

	platform := determineBuildClusterPlatform(clusterProfiles.ClusterPlatform, platformStr)

	return NewBuildClusterInstance0(platform), nil
}

func NewBuildClusterInstance0(p api.BuildPlatform) *BuildClusterInstance {
	return &BuildClusterInstance{platform: p}
}

type buildProxy struct {
	socketPath       string
	instance         *BuildClusterInstance
	listener         net.Listener
	cleanup          func() error
	useGrpcProxy     bool
	injectWorkerInfo *controlapi.ListWorkersResponse
}

func runBuildProxy(ctx context.Context, requestedPlatform api.BuildPlatform, socketPath string, connectAtStart, useGrpcProxy bool, workersInfo *controlapi.ListWorkersResponse) (*buildProxy, error) {
	bp, err := NewBuildClusterInstance(ctx, fmt.Sprintf("linux/%s", requestedPlatform))
	if err != nil {
		return nil, err
	}

	if connectAtStart {
		if c, err := bp.NewConn(ctx); err != nil {
			return nil, err
		} else {
			_ = c.Close()
		}
	}

	return bp.runBuildProxy(ctx, socketPath, useGrpcProxy, workersInfo)
}

func (bp *BuildClusterInstance) runBuildProxy(ctx context.Context, socketPath string, useGrpcProxy bool, workersInfo *controlapi.ListWorkersResponse) (*buildProxy, error) {
	var cleanup func() error
	if socketPath == "" {
		sockDir, err := dirs.CreateUserTempDir("", fmt.Sprintf("buildkit.%v", bp.platform))
		if err != nil {
			return nil, err
		}

		socketPath = filepath.Join(sockDir, "buildkit.sock")
		cleanup = func() error {
			return os.RemoveAll(sockDir)
		}
	} else {
		if err := unix.Unlink(socketPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	var d net.ListenConfig
	listener, err := d.Listen(ctx, "unix", socketPath)
	if err != nil {
		_ = bp.Cleanup()
		if cleanup != nil {
			_ = cleanup()
		}
		return nil, err
	}

	return &buildProxy{socketPath, bp, listener, cleanup, useGrpcProxy, workersInfo}, nil
}

func (bp *buildProxy) Cleanup() error {
	var errs []error
	errs = append(errs, bp.listener.Close())
	errs = append(errs, bp.instance.Cleanup())
	if bp.cleanup != nil {
		errs = append(errs, bp.cleanup())
	}
	return multierr.New(errs...)
}

func (bp *buildProxy) Serve(ctx context.Context) error {
	var err error
	sink := tasks.SinkFrom(ctx)
	if bp.useGrpcProxy {
		err = serveGRPCProxy(bp.injectWorkerInfo, bp.listener, func(innerCtx context.Context) (net.Conn, error) {
			return bp.instance.NewConn(tasks.WithSink(innerCtx, sink))
		})
	} else {
		err = serveProxy(ctx, bp.listener, func(innerCtx context.Context) (net.Conn, error) {
			return bp.instance.NewConn(tasks.WithSink(innerCtx, sink))
		})
	}

	if err != nil {
		if x, ok := err.(*net.OpError); ok {
			if x.Op == "accept" && errors.Is(x.Err, net.ErrClosed) {
				return nil
			}
		}

		return err
	}

	return nil
}

type buildProxyWithRegistry struct {
	BuildkitAddr    string
	DockerConfigDir string
	Cleanup         func() error
}

func runBuildProxyWithRegistry(ctx context.Context, platform api.BuildPlatform, nscrOnlyRegistry, useGrpcProxy bool, workerInfo *controlapi.ListWorkersResponse) (*buildProxyWithRegistry, error) {
	p, err := runBuildProxy(ctx, platform, "", true, useGrpcProxy, workerInfo)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := p.Serve(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	newConfig := *configfile.New(config.ConfigFileName)

	if !nscrOnlyRegistry {
		// This is a special option to support calling `nsc build --name`, which
		// implies that they want to use nscr.io registry, without asking the user to
		// set the credentials earlier with `nsc docker-login`.
		// In that case we cannot copy the CredentialsStore from default config
		// because MacOS Docker Engine would ignore the cleartext credentials we injected.
		existing := config.LoadDefaultConfigFile(console.Stderr(ctx))

		// We don't copy over all authentication settings; only some.
		// XXX replace with custom buildctl invocation that merges auth in-memory.

		newConfig.AuthConfigs = existing.AuthConfigs
		newConfig.CredentialHelpers = existing.CredentialHelpers
		newConfig.CredentialsStore = existing.CredentialsStore
	}

	nsRegs, err := api.GetImageRegistry(ctx, api.Methods)
	if err != nil {
		return nil, err
	}

	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return nil, err
	}
	for _, reg := range []*api.ImageRegistry{nsRegs.Registry, nsRegs.NSCR} {
		if reg != nil {
			newConfig.AuthConfigs[reg.EndpointAddress] = types.AuthConfig{
				ServerAddress: reg.EndpointAddress,
				Username:      nscrRegistryUsername,
				Password:      token.Raw(),
			}
		}
	}

	tmpDir := filepath.Dir(p.socketPath)
	credsFile := filepath.Join(tmpDir, config.ConfigFileName)
	if err := files.WriteJson(credsFile, newConfig, 0600); err != nil {
		p.Cleanup()
		return nil, err
	}

	return &buildProxyWithRegistry{p.socketPath, tmpDir, p.Cleanup}, nil
}
