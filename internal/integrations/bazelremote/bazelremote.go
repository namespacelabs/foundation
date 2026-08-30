// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	remoteclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	remotedigest "github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/filemetadata"
	"github.com/bazelbuild/remote-apis-sdks/go/pkg/rexec"
	asset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	execution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Config struct {
	executor     string
	cache        string
	remoteAsset  string
	instanceName string
	clientCert   string
	clientKey    string
	caCert       string
}

type ExecutorClient struct {
	*rexec.Client
	asset asset.FetchClient
}

func Executor(ctx context.Context) (*ExecutorClient, error) {
	return sharedExecutor(ctx)
}

var (
	executorMu sync.Mutex
	shared     *executorFuture
)

type executorFuture struct {
	ready    chan struct{}
	executor *ExecutorClient
	err      error
}

func sharedExecutor(ctx context.Context) (*ExecutorClient, error) {
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
		var assetClient asset.FetchClient
		grpcClient, assetClient, err = newClient(ctx, cfg)
		if err == nil {
			future.executor = &ExecutorClient{
				Client: &rexec.Client{GrpcClient: grpcClient, FileMetadataCache: filemetadata.NewSingleFlightCache()},
				asset:  assetClient,
			}
		}
	}
	future.err = err
	close(future.ready)
	return future.executor, future.err
}

func newClient(ctx context.Context, cfg Config) (*remoteclient.Client, asset.FetchClient, error) {
	// Dial separately so each endpoint receives its own TLS server name through SNI.
	executorParams, err := cfg.dialParams(cfg.executor)
	if err != nil {
		return nil, nil, err
	}
	executorOpts, _, err := remoteclient.OptsFromParams(ctx, executorParams)
	if err != nil {
		return nil, nil, err
	}
	executorConn, err := grpc.DialContext(ctx, executorParams.Service, executorOpts...)
	if err != nil {
		return nil, nil, err
	}

	cacheParams, err := cfg.dialParams(cfg.cache)
	if err != nil {
		return nil, nil, errors.Join(err, executorConn.Close())
	}
	assetParams, err := cfg.dialParams(cfg.remoteAsset)
	if err != nil {
		return nil, nil, errors.Join(err, executorConn.Close())
	}
	if assetParams.Service != cacheParams.Service || assetParams.NoSecurity != cacheParams.NoSecurity {
		return nil, nil, errors.Join(fmt.Errorf("remote asset endpoint %q does not match storage endpoint %q", cfg.remoteAsset, cfg.cache), executorConn.Close())
	}
	cacheOpts, _, err := remoteclient.OptsFromParams(ctx, cacheParams)
	if err != nil {
		return nil, nil, errors.Join(err, executorConn.Close())
	}
	cacheConn, err := grpc.DialContext(ctx, cacheParams.Service, cacheOpts...)
	if err != nil {
		return nil, nil, errors.Join(err, executorConn.Close())
	}

	// Namespace serves capabilities from storage, while this SDK probes the execution connection.
	client, err := remoteclient.NewClientFromConnection(ctx, cfg.instanceName, executorConn, cacheConn, remoteclient.StartupCapabilities(false))
	if err != nil {
		return nil, nil, errors.Join(err, executorConn.Close(), cacheConn.Close())
	}
	return client, asset.NewFetchClient(cacheConn), nil
}

func (c *ExecutorClient) FetchBlob(ctx context.Context, uri, sha256Hex string) (remotedigest.Digest, error) {
	checksum, err := hex.DecodeString(sha256Hex)
	if err != nil || len(checksum) != 32 {
		return remotedigest.Digest{}, fmt.Errorf("invalid SHA-256 digest %q", sha256Hex)
	}
	response, err := c.asset.FetchBlob(ctx, &asset.FetchBlobRequest{
		Uris:           []string{uri},
		DigestFunction: execution.DigestFunction_SHA256,
		Qualifiers: []*asset.Qualifier{{
			Name:  "checksum.sri",
			Value: "sha256-" + base64.StdEncoding.EncodeToString(checksum),
		}},
	})
	if err != nil {
		return remotedigest.Digest{}, err
	}
	if response == nil || response.GetStatus() == nil {
		return remotedigest.Digest{}, fmt.Errorf("remote asset fetch returned an empty response")
	}
	if response.GetStatus().GetCode() != int32(codes.OK) {
		return remotedigest.Digest{}, status.FromProto(response.GetStatus()).Err()
	}
	digest, err := remotedigest.NewFromProto(response.GetBlobDigest())
	if err != nil {
		return remotedigest.Digest{}, err
	}
	if digest.Hash != sha256Hex {
		return remotedigest.Digest{}, fmt.Errorf("remote asset fetch returned digest %q, expected %q", digest.Hash, sha256Hex)
	}
	return digest, nil
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
