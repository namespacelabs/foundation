// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
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

type ProxyStatus string

const (
	StartingProxyStatus = "Starting"
	RunningProxyStatus  = "Running"
	FailingProxyStatus  = "Failing"
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

func (bp *BuildClusterInstance) NewConn(parentCtx context.Context) (net.Conn, string, error) {
	// Wait at most 20 seconds to create a connection to a build cluster.
	ctx, done := context.WithTimeout(parentCtx, 20*time.Second)
	defer done()

	// This is not our usual play; we're doing a lot of work with the lock held.
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.previous != nil && (bp.previous.BuildCluster == nil || bp.previous.BuildCluster.Resumable) {
		if _, err := api.EnsureCluster(ctx, api.Methods, bp.previous.ClusterId); err == nil {
			conn, err := bp.rawDial(ctx, bp.previous)
			return conn, bp.previous.ClusterId, err
		}
	}

	response, err := api.CreateBuildCluster(ctx, api.Methods, bp.platform)
	if err != nil {
		return nil, "", err
	}

	if bp.cancelRefresh != nil {
		bp.cancelRefresh()
		bp.cancelRefresh = nil
	}

	if bp.previous == nil || bp.previous.ClusterId != response.ClusterId {
		if err := waitUntilReady(ctx, response); err != nil {
			return nil, "", fmt.Errorf("failed to wait for buildkit to become ready: %w", err)
		}
	}

	if response.BuildCluster != nil && !response.BuildCluster.DoesNotRequireRefresh {
		bp.cancelRefresh = api.StartBackgroundRefreshing(parentCtx, response.ClusterId)
	}

	bp.previous = response

	conn, err := bp.rawDial(ctx, response)
	return conn, bp.previous.ClusterId, err
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
	socketPath        string
	controlSocketPath string
	instance          *BuildClusterInstance
	listener          net.Listener
	cleanup           func() error
	useGrpcProxy      bool
	injectWorkerInfo  *controlapi.ListWorkersResponse
	proxyStatus       *proxyStatusDesc
}

// proxyStatus is used by `nsc docker buildx status` to show user info on
// proxy current status
type proxyStatusDesc struct {
	mu                sync.RWMutex
	Platform          string
	Status            ProxyStatus
	LastError         string
	LogPath           string
	BuilderID         string
	PreviousBuilderID string
	LastUpdate        time.Time
	Requests          int
}

func (p *proxyStatusDesc) setBuilderID(status ProxyStatus, builderID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
	p.LastUpdate = time.Now()
	p.PreviousBuilderID = p.BuilderID
	p.BuilderID = builderID
}

func (p *proxyStatusDesc) setStatus(status ProxyStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
	p.LastUpdate = time.Now()
}

func (p *proxyStatusDesc) setLastError(status ProxyStatus, lastError error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = status
	p.LastUpdate = time.Now()
	p.LastError = lastError.Error()
}

func (p *proxyStatusDesc) incRequest() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Requests++
	p.LastUpdate = time.Now()
}

func runBuildProxy(ctx context.Context, requestedPlatform api.BuildPlatform, socketPath, controlSocketPath string, connectAtStart, useGrpcProxy bool, workersInfo *controlapi.ListWorkersResponse) (*buildProxy, error) {
	bp, err := NewBuildClusterInstance(ctx, fmt.Sprintf("linux/%s", requestedPlatform))
	if err != nil {
		return nil, err
	}

	if connectAtStart {
		if c, _, err := bp.NewConn(ctx); err != nil {
			return nil, err
		} else {
			_ = c.Close()
		}
	}

	return bp.runBuildProxy(ctx, socketPath, controlSocketPath, useGrpcProxy, workersInfo)
}

func (bp *BuildClusterInstance) runBuildProxy(ctx context.Context, socketPath, controlSocketPath string, useGrpcProxy bool, workersInfo *controlapi.ListWorkersResponse) (*buildProxy, error) {
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

	status := &proxyStatusDesc{
		Status:   StartingProxyStatus,
		Platform: string(bp.platform),
		LogPath:  console.DebugToFile,
	}

	if bp.previous != nil {
		status.BuilderID = bp.previous.ClusterId
	}

	return &buildProxy{socketPath, controlSocketPath, bp, listener, cleanup, useGrpcProxy, workersInfo, status}, nil
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
		err = serveGRPCProxy(bp.injectWorkerInfo, bp.listener, bp.proxyStatus, func(innerCtx context.Context) (net.Conn, string, error) {
			return bp.instance.NewConn(tasks.WithSink(innerCtx, sink))
		})
	} else {
		err = serveProxy(ctx, bp.listener, func(innerCtx context.Context) (net.Conn, error) {
			conn, _, err := bp.instance.NewConn(tasks.WithSink(innerCtx, sink))
			return conn, err
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

func (bp *buildProxy) ServeStatus(ctx context.Context) error {
	if err := unix.Unlink(bp.controlSocketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	l, err := net.Listen("unix", bp.controlSocketPath)
	if err != nil {
		return err
	}
	defer l.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		bp.proxyStatus.mu.RLock()
		defer bp.proxyStatus.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(bp.proxyStatus); err != nil {
			fmt.Fprintf(console.Stderr(ctx), "Http Server error: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	})

	return http.Serve(l, mux)
}

type buildProxyWithRegistry struct {
	BuildkitAddr    string
	DockerConfigDir string
	Cleanup         func() error
}

func runBuildProxyWithRegistry(ctx context.Context, platform api.BuildPlatform, nscrOnlyRegistry, useGrpcProxy bool, workerInfo *controlapi.ListWorkersResponse) (*buildProxyWithRegistry, error) {
	p, err := runBuildProxy(ctx, platform, "", "", true, useGrpcProxy, workerInfo)
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
