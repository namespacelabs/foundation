// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	executionv2 "buf.build/gen/go/namespace/bazel/protocolbuffers/go/build/bazel/remote/execution/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	bazelCacheReadinessTimeout        = time.Minute
	bazelCacheReadinessAttemptTimeout = 2 * time.Second
	bazelCacheReadinessRetryWait      = 200 * time.Millisecond
	bazelGetCapabilitiesMethod        = "/build.bazel.remote.execution.v2.Capabilities/GetCapabilities"
)

type bazelCacheReadinessConfig struct {
	endpoint    string
	serverCA    string
	clientCert  string
	clientKey   string
	bearerToken string
	waitTimeout time.Duration
}

type parsedBazelCacheEndpoint struct {
	target     string
	serverName string
	statusURL  string
	secure     bool
}

func waitForBazelCacheReady(ctx context.Context, cfg bazelCacheReadinessConfig) error {
	waitCtx, cancel := context.WithTimeout(ctx, cfg.waitTimeout)
	defer cancel()

	var lastErr error
	for {
		attemptCtx, cancel := context.WithTimeout(waitCtx, bazelCacheReadinessAttemptTimeout)
		lastErr = checkBazelCacheReady(attemptCtx, cfg)
		cancel()
		if lastErr == nil {
			return nil
		}

		timer := time.NewTimer(bazelCacheReadinessRetryWait)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			if err := ctx.Err(); err != nil {
				return err
			}
			return fmt.Errorf("cache endpoint %q did not become ready within %s: %w", cfg.endpoint, cfg.waitTimeout, lastErr)
		case <-timer.C:
		}
	}
}

func checkBazelCacheReady(ctx context.Context, cfg bazelCacheReadinessConfig) error {
	endpoint, err := parseBazelCacheEndpoint(cfg.endpoint)
	if err != nil {
		return err
	}

	var tlsConfig *tls.Config
	if endpoint.secure {
		tlsConfig = &tls.Config{ServerName: endpoint.serverName}
		if cfg.serverCA != "" {
			rootCAs := x509.NewCertPool()
			serverCA, err := os.ReadFile(cfg.serverCA)
			if err != nil {
				return fmt.Errorf("read Bazel cache server CA: %w", err)
			}
			if !rootCAs.AppendCertsFromPEM(serverCA) {
				return fmt.Errorf("parse Bazel cache server CA")
			}
			tlsConfig.RootCAs = rootCAs
		}
		if cfg.clientCert != "" || cfg.clientKey != "" {
			if cfg.clientCert == "" || cfg.clientKey == "" {
				return fmt.Errorf("both Bazel cache client certificate and key are required")
			}
			clientCertificate, err := tls.LoadX509KeyPair(cfg.clientCert, cfg.clientKey)
			if err != nil {
				return fmt.Errorf("load Bazel cache client certificate: %w", err)
			}
			tlsConfig.Certificates = []tls.Certificate{clientCertificate}
		}
	}

	if endpoint.statusURL != "" {
		return checkBazelHTTPCacheReady(ctx, endpoint.statusURL, tlsConfig, cfg.bearerToken)
	}

	var transportCredentials credentials.TransportCredentials
	if endpoint.secure {
		transportCredentials = credentials.NewTLS(tlsConfig)
	} else {
		transportCredentials = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(endpoint.target, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		return err
	}
	defer conn.Close()

	if cfg.bearerToken != "" {
		bearer := "Bearer " + cfg.bearerToken
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", bearer, "x-nsc-ingress-auth", bearer)
	}

	return conn.Invoke(
		ctx,
		bazelGetCapabilitiesMethod,
		&executionv2.GetCapabilitiesRequest{},
		&executionv2.ServerCapabilities{},
		grpc.WaitForReady(true),
	)
}

func checkBazelHTTPCacheReady(ctx context.Context, statusURL string, tlsConfig *tls.Config, bearerToken string) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	defer transport.CloseIdleConnections()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return err
	}
	if bearerToken != "" {
		bearer := "Bearer " + bearerToken
		request.Header.Set("Authorization", bearer)
		request.Header.Set("x-nsc-ingress-auth", bearer)
	}

	response, err := transport.RoundTrip(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("cache status endpoint returned %s", response.Status)
	}
	return nil
}

func parseBazelCacheEndpoint(endpoint string) (parsedBazelCacheEndpoint, error) {
	value := endpoint
	if !strings.Contains(value, "://") {
		value = "grpcs://" + value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return parsedBazelCacheEndpoint{}, fmt.Errorf("parse Bazel cache endpoint: %w", err)
	}

	var secure, useHTTP bool
	switch parsed.Scheme {
	case "grpc", "http":
		secure = false
		useHTTP = parsed.Scheme == "http"
	case "grpcs", "https":
		secure = true
		useHTTP = parsed.Scheme == "https"
	default:
		return parsedBazelCacheEndpoint{}, fmt.Errorf("unsupported Bazel cache endpoint scheme %q", parsed.Scheme)
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return parsedBazelCacheEndpoint{}, fmt.Errorf("Bazel cache endpoint must not contain credentials, a path, query parameters, or a fragment")
	}

	serverName := parsed.Hostname()
	if serverName == "" {
		return parsedBazelCacheEndpoint{}, fmt.Errorf("Bazel cache endpoint is missing a host")
	}
	port := parsed.Port()
	if port == "" {
		if secure {
			port = "443"
		} else {
			port = "80"
		}
	}

	result := parsedBazelCacheEndpoint{
		target:     net.JoinHostPort(serverName, port),
		secure:     secure,
		serverName: serverName,
	}
	if useHTTP {
		parsed.Path = "/status"
		result.statusURL = parsed.String()
	}
	return result, nil
}
