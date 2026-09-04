// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	bazelv1betagrpc "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/cloud/integrations/bazel/v1beta/bazelv1betagrpc"
	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
	remoteclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	execution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/integrations/nsc/grpcapi"
)

const (
	provisioningAttempts = 4
	provisioningWait     = 200 * time.Millisecond
	readinessTimeout     = time.Minute
	readinessAttempt     = 2 * time.Second
)

func setup(ctx context.Context) (_ Config, retErr error) {
	token, err := fnapi.FetchToken(ctx)
	if err != nil {
		return Config{}, err
	}
	conn, err := grpcapi.NewConnectionWithEndpoint(ctx, fnapi.GlobalEndpoint(), token)
	if err != nil {
		return Config{}, err
	}
	defer func() {
		retErr = errors.Join(retErr, conn.Close())
	}()

	request := &bazelv1beta.EnsureClusterRequest{}
	request.SetAuthMode(bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_MTLS)
	request.SetEnableRemoteAssetApi(true)
	response, err := ensureCluster(ctx, bazelv1betagrpc.NewBazelServiceClient(conn), request)
	if err != nil {
		return Config{}, fnerrors.Newf("failed to provision Bazel execution cluster: %w", err)
	}
	if response.GetSchedulerEndpoint() == "" || response.GetStorageEndpoint() == "" || response.GetRemoteAssetEndpoint() == "" {
		return Config{}, fnerrors.Newf("received incomplete Bazel execution cluster response (scheduler=%q storage=%q remote_asset=%q)", response.GetSchedulerEndpoint(), response.GetStorageEndpoint(), response.GetRemoteAssetEndpoint())
	}

	privateKey, publicKey, err := clientKeyPair()
	if err != nil {
		return Config{}, fnerrors.Newf("failed to generate Bazel client key pair: %w", err)
	}
	clientCertificate, err := fnapi.IssueTenantClientCertFromToken(ctx, token, string(publicKey))
	if err != nil {
		return Config{}, fnerrors.Newf("failed to issue Bazel client certificate: %w", err)
	}
	clientCertPath, err := writeCredential("*.cert", []byte(clientCertificate))
	if err != nil {
		return Config{}, err
	}
	clientKeyPath, err := writeCredential("*.key", privateKey)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		executor:    response.GetSchedulerEndpoint(),
		cache:       response.GetStorageEndpoint(),
		remoteAsset: response.GetRemoteAssetEndpoint(),
		clientCert:  clientCertPath,
		clientKey:   clientKeyPath,
	}
	if err := waitForCache(ctx, cfg); err != nil {
		return Config{}, fnerrors.Newf("failed waiting for Bazel storage readiness: %w", err)
	}
	return cfg, nil
}

func ensureCluster(ctx context.Context, client bazelv1betagrpc.BazelServiceClient, request *bazelv1beta.EnsureClusterRequest) (*bazelv1beta.EnsureClusterResponse, error) {
	var response *bazelv1beta.EnsureClusterResponse
	var err error
	for attempt := 0; attempt < provisioningAttempts; attempt++ {
		response, err = client.EnsureCluster(ctx, request)
		if err == nil || !retryableProvisioningError(err) {
			return response, err
		}
		if attempt+1 == provisioningAttempts {
			return nil, err
		}
		timer := time.NewTimer(provisioningWait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, err
}

func retryableProvisioningError(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.Aborted, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

func clientKeyPair() ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyBytes}), nil
}

func writeCredential(pattern string, contents []byte) (string, error) {
	file, err := dirs.CreateUserTemp("bazelrbe", pattern)
	if err != nil {
		return "", fnerrors.Newf("failed to create Bazel credential file: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		return "", fnerrors.Newf("failed to write Bazel credential file: %w", errors.Join(err, file.Close()))
	}
	if err := file.Close(); err != nil {
		return "", fnerrors.Newf("failed to close Bazel credential file: %w", err)
	}
	return file.Name(), nil
}

func waitForCache(ctx context.Context, cfg Config) (retErr error) {
	params, err := cfg.dialParams(cfg.cache)
	if err != nil {
		return err
	}
	opts, _, err := remoteclient.OptsFromParams(ctx, params)
	if err != nil {
		return err
	}
	conn, err := grpc.DialContext(ctx, params.Service, opts...)
	if err != nil {
		return err
	}
	defer func() {
		retErr = errors.Join(retErr, conn.Close())
	}()

	waitCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	client := execution.NewCapabilitiesClient(conn)
	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(waitCtx, readinessAttempt)
		_, lastErr = client.GetCapabilities(attemptCtx, &execution.GetCapabilitiesRequest{}, grpc.WaitForReady(true))
		cancel()
		if lastErr == nil {
			return nil
		}
		timer := time.NewTimer(provisioningWait)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			if err := ctx.Err(); err != nil {
				return err
			}
			return fmt.Errorf("cache endpoint %q did not become ready within %s: %w", cfg.cache, readinessTimeout, lastErr)
		case <-timer.C:
		}
	}
}
