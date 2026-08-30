// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	remoteclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/rexec"
	"google.golang.org/grpc"
)

type Config struct {
	executor     string
	cache        string
	instanceName string
	clientCert   string
	clientKey    string
	caCert       string
}

func Executor(ctx context.Context) (*rexec.Client, error) {
	return sharedExecutor(ctx)
}

var (
	executorMu sync.Mutex
	shared     *executorFuture
)

type executorFuture struct {
	ready    chan struct{}
	executor *rexec.Client
	err      error
}

func sharedExecutor(ctx context.Context) (*rexec.Client, error) {
	executorMu.Lock()
	future := shared
	created := false
	if future == nil {
		future = &executorFuture{ready: make(chan struct{})}
		shared = future
		created = true
	}
	executorMu.Unlock()

	if !created {
		select {
		case <-future.ready:
			return future.executor, future.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	cfg, err := setup(ctx)
	if err == nil {
		var grpcClient *remoteclient.Client
		grpcClient, err = newClient(ctx, cfg)
		if err == nil {
			future.executor = &rexec.Client{GrpcClient: grpcClient, FileMetadataCache: filemetadata.NewSingleFlightCache()}
		}
	}
	future.err = err
	close(future.ready)
	return future.executor, future.err
}

func newClient(ctx context.Context, cfg Config) (*remoteclient.Client, error) {
	// Dial separately so each endpoint receives its own TLS server name through SNI.
	executorParams, err := cfg.dialParams(cfg.executor)
	if err != nil {
		return nil, err
	}
	executorOpts, _, err := remoteclient.OptsFromParams(ctx, executorParams)
	if err != nil {
		return nil, err
	}
	executorConn, err := grpc.DialContext(ctx, executorParams.Service, executorOpts...)
	if err != nil {
		return nil, err
	}

	cacheParams, err := cfg.dialParams(cfg.cache)
	if err != nil {
		return nil, errors.Join(err, executorConn.Close())
	}
	cacheOpts, _, err := remoteclient.OptsFromParams(ctx, cacheParams)
	if err != nil {
		return nil, errors.Join(err, executorConn.Close())
	}
	cacheConn, err := grpc.DialContext(ctx, cacheParams.Service, cacheOpts...)
	if err != nil {
		return nil, errors.Join(err, executorConn.Close())
	}

	// Namespace serves capabilities from storage, while this SDK probes the execution connection.
	client, err := remoteclient.NewClientFromConnection(ctx, cfg.instanceName, executorConn, cacheConn, remoteclient.StartupCapabilities(false))
	if err != nil {
		return nil, errors.Join(err, executorConn.Close(), cacheConn.Close())
	}
	return client, nil
}

func (c Config) dialParams(endpoint string) (remoteclient.DialParams, error) {
	target, secure, err := grpcTarget(endpoint)
	if err != nil {
		return remoteclient.DialParams{}, err
	}
	return remoteclient.DialParams{
		Service:           target,
		NoSecurity:        !secure,
		NoAuth:            true,
		TLSCACertFile:     c.caCert,
		TLSClientAuthCert: c.clientCert,
		TLSClientAuthKey:  c.clientKey,
	}, nil
}

func grpcTarget(raw string) (string, bool, error) {
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("parse Bazel remote endpoint: %w", err)
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("Bazel remote endpoint %q is missing a host", raw)
	}
	if u.Scheme != "grpc" && u.Scheme != "grpcs" {
		return "", false, fmt.Errorf("Bazel remote endpoint %q has unsupported scheme %q", raw, u.Scheme)
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", false, fmt.Errorf("Bazel remote endpoint %q must not contain credentials, a path, query parameters, or a fragment", raw)
	}
	return u.Host, u.Scheme == "grpcs", nil
}
