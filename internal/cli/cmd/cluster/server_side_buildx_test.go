// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	controlapi "github.com/moby/buildkit/api/services/control"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func TestWaitForBuildxBuildersSelectedPlatforms(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		platforms []string
		attempts  [2]int32
	}{
		{"arm64 only", []string{"linux/arm64"}, [2]int32{0, 3}},
		{"both", []string{"amd64", "linux/arm64"}, [2]int32{2, 3}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cmd := newSetupBuildxCmd()
			args := []string{"--wait-for-builder", "--tag=custom-builder"}
			for _, plat := range test.platforms {
				args = append(args, "--platform="+plat)
			}
			if err := cmd.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			waitTimeout, err := cmd.Flags().GetDuration("wait-for-builder")
			if err != nil || waitTimeout != 5*time.Minute {
				t.Fatalf("wait timeout = %v, error = %v, want 5m", waitTimeout, err)
			}
			if tag, _ := cmd.Flags().GetString("tag"); tag != "custom-builder" {
				t.Fatalf("tag = %q, want custom-builder", tag)
			}
			requested, _ := cmd.Flags().GetStringArray("platform")
			selected, err := selectBuildxPlatforms([]api.BuildPlatform{"amd64", "arm64"}, requested)
			if err != nil {
				t.Fatal(err)
			}

			var attempts [2]atomic.Int32
			configs := map[api.BuildPlatform]BuilderConfig{}
			for i, arch := range []api.BuildPlatform{"amd64", "arm64"} {
				configs[arch] = startBuildxReadinessServer(t, "linux/"+string(arch), func(context.Context) error {
					if attempts[i].Add(1) < int32(i+2) {
						return status.Error(codes.Unavailable, "starting")
					}
					return nil
				})
			}
			var selectedConfigs []BuilderConfig
			for _, plat := range selected {
				selectedConfigs = append(selectedConfigs, configs[plat])
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := waitForBuildxBuilders(ctx, selectedConfigs, waitTimeout); err != nil {
				t.Fatal(err)
			}
			for i := range attempts {
				if got := attempts[i].Load(); got != test.attempts[i] {
					t.Errorf("builder %d: attempts = %d, want %d", i, got, test.attempts[i])
				}
			}
		})
	}
}

func TestWaitForBuildxBuildersAuthenticationFailure(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	cfg := startBuildxReadinessServer(t, "linux/arm64", func(context.Context) error {
		attempts.Add(1)
		return status.Error(codes.PermissionDenied, "denied")
	})
	err := waitForBuildxBuilders(context.Background(), []BuilderConfig{cfg}, 5*time.Minute)
	if status.Code(err) != codes.PermissionDenied || !strings.Contains(err.Error(), "linux/arm64") {
		t.Fatalf("error = %v, want platform-specific permission error", err)
	}
	if got := attempts.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
}

func TestWaitForBuildxBuildersDeadline(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		parentTimeout time.Duration
		waitTimeout   time.Duration
		parentExpired bool
	}{
		{"configured timeout", 5 * time.Second, 100 * time.Millisecond, false},
		{"parent deadline", 100 * time.Millisecond, 5 * time.Minute, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			cfg := startBuildxReadinessServer(t, "linux/amd64", func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			})
			ctx, cancel := context.WithTimeout(context.Background(), test.parentTimeout)
			defer cancel()
			err := waitForBuildxBuilders(ctx, []BuilderConfig{cfg}, test.waitTimeout)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("error = %v, want deadline exceeded", err)
			}
			if expired := ctx.Err() != nil; expired != test.parentExpired {
				t.Fatalf("parent expired = %t, want %t", expired, test.parentExpired)
			}
		})
	}
}

func TestSetupBuildxWaitTimeoutFlags(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		args    []string
		timeout time.Duration
	}{
		{"disabled", nil, 0},
		{"default", []string{"--wait-for-builder"}, 5 * time.Minute},
		{"custom", []string{"--wait-for-builder=30s"}, 30 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := newSetupBuildxCmd()
			args := append(test.args, "--tag=custom-builder", "--platform=linux/arm64")
			if err := cmd.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if got, err := cmd.Flags().GetDuration("wait-for-builder"); err != nil || got != test.timeout {
				t.Errorf("timeout = %v, error = %v, want %v", got, err, test.timeout)
			}
			if got, _ := cmd.Flags().GetString("tag"); got != "custom-builder" {
				t.Errorf("tag = %q, want custom-builder", got)
			}
			if got, _ := cmd.Flags().GetStringArray("platform"); len(got) != 1 || got[0] != "linux/arm64" {
				t.Errorf("platforms = %v, want [linux/arm64]", got)
			}
			if err := cmd.ValidateArgs(cmd.Flags().Args()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetupBuildxWaitRejectsInvalidTimeout(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		args    []string
		wantErr string
	}{
		{[]string{"--wait-for-builder=0s"}, "--wait-for-builder must be greater than zero"},
		{[]string{"--wait-for-builder=-1s"}, "--wait-for-builder must be greater than zero"},
		{[]string{"--wait-for-builder=invalid"}, "invalid duration"},
		{[]string{"--wait-for-builder", "30s"}, "unknown command"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			cmd := newSetupBuildxCmd()
			cmd.SetArgs(test.args)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestSetupBuildxWaitRejectsClientSideProxy(t *testing.T) {
	t.Parallel()

	cmd := newSetupBuildxCmd()
	cmd.SetArgs([]string{"--wait-for-builder", "--platform=linux/arm64", "--use_server_side_proxy=false"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--wait-for-builder requires --use_server_side_proxy") {
		t.Fatalf("error = %v, want incompatible proxy flags", err)
	}
}

func TestSetupBuildxWaitRequiresPlatform(t *testing.T) {
	t.Parallel()

	cmd := newSetupBuildxCmd()
	cmd.SetArgs([]string{"--wait-for-builder"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--wait-for-builder requires --platform") {
		t.Fatalf("error = %v, want missing platform error", err)
	}
}

func TestSetupBuildxClientSideProxyIgnoresUnchangedWait(t *testing.T) {
	t.Parallel()

	cmd := newSetupBuildxCmd()
	// Set a nonzero value without marking the flag as explicitly supplied.
	if err := cmd.Flags().Lookup("wait-for-builder").Value.Set("5m"); err != nil {
		t.Fatal(err)
	}
	// Stop at the next validation, before any setup or API calls.
	cmd.SetArgs([]string{"--use_server_side_proxy=false", "--background_debug_dir=test"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "--background_debug_dir requires --background") {
		t.Fatalf("error = %v, want to proceed past wait-for-builder validation", err)
	}
	if cmd.Flags().Changed("wait-for-builder") {
		t.Fatal("wait-for-builder should remain unchanged")
	}
}

func startBuildxReadinessServer(t *testing.T, platform string, check func(context.Context) error) BuilderConfig {
	t.Helper()

	// Reuse httptest's localhost certificate for the TLS gRPC endpoint.
	httpServer := httptest.NewTLSServer(nil)
	cert := httpServer.TLS.Certificates[0]
	httpServer.Close()
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(t.TempDir(), "cert.pem")
	contents := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	contents = append(contents, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})...)
	if err := os.WriteFile(certPath, contents, 0600); err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAnyClientCert})),
		grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
			method, _ := grpc.Method(stream.Context())
			if method == "/moby.buildkit.v1.Control/ListWorkers" {
				return stream.SendMsg(&controlapi.ListWorkersResponse{})
			}
			if method != "/moby.buildkit.v1.Control/DiskUsage" {
				return status.Errorf(codes.Unimplemented, "unexpected method %q", method)
			}
			if err := stream.RecvMsg(&controlapi.DiskUsageRequest{}); err != nil {
				return err
			}
			if err := check(stream.Context()); err != nil {
				return err
			}
			return stream.SendMsg(&controlapi.DiskUsageResponse{})
		}),
	)
	t.Cleanup(func() {
		server.Stop()
		listener.Close()
	})
	go func() { _ = server.Serve(listener) }()
	return BuilderConfig{
		Platform:             platform,
		FullBuildkitEndpoint: "tcp://" + listener.Addr().String(),
		ServerCAPath:         certPath,
		ClientCertPath:       certPath,
		ClientKeyPath:        certPath,
	}
}
