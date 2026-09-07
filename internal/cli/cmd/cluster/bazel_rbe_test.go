// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
	"connectrpc.com/connect"
	"github.com/cenkalti/backoff/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestToBazelExecutionConfigBuildEventsStatic(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		TenantID:           "tenant_test",
		SchedulerEndpoint:  "grpcs://scheduler.example:443",
		StorageEndpoint:    "grpcs://storage.example:443",
		IngressAuthToken:   "tok123",
		BuildEventEndpoint: "grpcs://api.us-east1.namespaceapis.com",
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
		"build --bes_results_url=https://cloud.namespace.so/tenant_test/bazel/invocation/\n",
		"build --bes_header=Authorization=Bearer\\ tok123\n",
		"build --bes_header=x-nsc-ingress-auth=Bearer\\ tok123\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "credential_helper") {
		t.Fatalf("static mode must not configure the credential helper: %q", got)
	}
}

func TestToBazelExecutionConfigBuildEventsMTLS(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		TenantID:                "tenant_test",
		SchedulerEndpoint:       "grpcs://scheduler.example:444",
		StorageEndpoint:         "grpcs://storage.example:444",
		ClientCert:              "/tmp/client.cert",
		ClientKey:               "/tmp/client.key",
		BuildEventEndpoint:      "grpcs://api.us-east1.namespaceapis.com",
		CredentialHelperDomains: []string{"api.us-east1.namespaceapis.com"},
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
		"build --bes_results_url=https://cloud.namespace.so/tenant_test/bazel/invocation/\n",
		"build --credential_helper=*.api.us-east1.namespaceapis.com=" + BazelCredHelperBinary + "\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	if strings.Contains(got, "--bes_header=") {
		t.Fatalf("mTLS mode must not configure bes headers: %q", got)
	}
}

func TestToBazelExecutionConfigNoBuildEvents(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		TenantID:          "tenant_test",
		SchedulerEndpoint: "grpcs://scheduler.example:444",
		StorageEndpoint:   "grpcs://storage.example:444",
		ClientCert:        "/tmp/client.cert",
		ClientKey:         "/tmp/client.key",
	}, "build", true, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	if strings.Contains(got, "--bes_backend") || strings.Contains(got, "--bes_results_url") || strings.Contains(got, "credential_helper") {
		t.Fatalf("must not configure build events when no endpoint is returned: %q", got)
	}
}

func TestToBazelExecutionConfigBuildEventsDisabled(t *testing.T) {
	t.Parallel()

	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		TenantID:                "tenant_test",
		SchedulerEndpoint:       "grpcs://scheduler.example:444",
		StorageEndpoint:         "grpcs://storage.example:444",
		BuildEventEndpoint:      "grpcs://api.us-east1.namespaceapis.com",
		CredentialHelperDomains: []string{"api.us-east1.namespaceapis.com"},
	}, "build", true, true)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, unwanted := range []string{"--bes_backend", "--bes_results_url", "--bes_header", "credential_helper"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("disabled build event config contains %q: %q", unwanted, got)
		}
	}
	if !strings.Contains(got, "build --remote_cache=grpcs://storage.example:444\n") {
		t.Fatalf("disabled build events removed remote cache config: %q", got)
	}
}

func TestToBazelExecutionConfigWithoutRemoteExecution(t *testing.T) {
	t.Parallel()

	remoteUploadLocalResults := false
	config, err := toBazelExecutionConfig(context.Background(), bazelRbeSetup{
		TenantID:                 "tenant_test",
		SchedulerEndpoint:        "grpcs://scheduler.example:443",
		StorageEndpoint:          "grpcs://storage.example:443",
		RemoteUploadLocalResults: &remoteUploadLocalResults,
		IngressAuthToken:         "tok123",
		RemoteLocalFallback:      true,
		RemoteDownloadOutputs:    "minimal",
		RemoteTimeout:            5 * time.Minute,
		Jobs:                     32,
		BuildEventEndpoint:       "grpcs://api.us-east1.namespaceapis.com",
	}, "build", false, false)
	if err != nil {
		t.Fatalf("toBazelExecutionConfig: %v", err)
	}

	got := string(config)
	for _, want := range []string{
		"build --remote_cache=grpcs://storage.example:443\n",
		"build --remote_header=x-nsc-ingress-auth=Bearer\\ tok123\n",
		"build --remote_download_outputs=minimal\n",
		"build --remote_upload_local_results=false\n",
		"build --jobs=32\n",
		"build --remote_timeout=300\n",
		"build --bes_backend=grpcs://api.us-east1.namespaceapis.com\n",
		"build --bes_results_url=https://cloud.namespace.so/tenant_test/bazel/invocation/\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing config line %q in %q", want, got)
		}
	}

	for _, unwanted := range []string{
		"--remote_executor=",
		"--spawn_strategy=remote",
		"--genrule_strategy=remote",
		"--remote_local_fallback=",
	} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("remote execution config %q present in %q", unwanted, got)
		}
	}
}

func TestNewBazelCmdSetupAlias(t *testing.T) {
	t.Parallel()

	cmd := NewBazelCmd()
	setup, _, err := cmd.Find([]string{"setup"})
	if err != nil {
		t.Fatalf("finding bazel setup: %v", err)
	}
	if !setup.Hidden {
		t.Fatal("bazel setup must be hidden")
	}

	remote := setup.Flags().Lookup("remote")
	if remote == nil {
		t.Fatal("bazel setup is missing --remote")
	}
	if remote.DefValue != "true" {
		t.Fatalf("--remote default = %q, want true", remote.DefValue)
	}
	storage := setup.Flags().Lookup("storage")
	if storage == nil {
		t.Fatal("bazel setup is missing --storage")
	}
	if storage.DefValue != bazelStorageReadWrite {
		t.Fatalf("--storage default = %q, want %q", storage.DefValue, bazelStorageReadWrite)
	}
	if setup.Flags().Lookup("token") == nil {
		t.Fatal("bazel setup is missing --token")
	}
	if setup.Flags().Lookup("disable_build_events") == nil {
		t.Fatal("bazel setup is missing --disable_build_events")
	}

	executionSetup, _, err := cmd.Find([]string{"execution", "setup"})
	if err != nil {
		t.Fatalf("finding bazel execution setup: %v", err)
	}
	if executionSetup.Flags().Lookup("remote") != nil {
		t.Fatal("bazel execution setup must not expose --remote")
	}
	if executionSetup.Flags().Lookup("storage") != nil {
		t.Fatal("bazel execution setup must not expose --storage")
	}
	if executionSetup.Flags().Lookup("token") == nil {
		t.Fatal("bazel execution setup is missing --token")
	}
	if executionSetup.Flags().Lookup("disable_build_events") == nil {
		t.Fatal("bazel execution setup is missing --disable_build_events")
	}
}

func TestBazelSetupStorageModeValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "invalid mode", args: []string{"--storage=invalid"}, want: "invalid storage mode"},
		{name: "read only with remote execution", args: []string{"--storage=read-only"}, want: "requires --remote=false"},
		{name: "read only with remote asset API", args: []string{"--remote=false", "--storage=read-only", "--enable_remote_asset_api"}, want: "may not be used with --storage=read-only"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newSetupBazelCmd()
			cmd.SetArgs(tc.args)
			err := cmd.ExecuteContext(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ExecuteContext() error = %v, want error containing %q", err, tc.want)
			}
		})
	}
}

func TestBazelStorageSetup(t *testing.T) {
	t.Parallel()

	response := &bazelv1beta.EnsureStorageClusterResponse{
		StorageEndpoint:                          "grpcs://storage.example:444",
		RemoteAssetEndpoint:                      "grpcs://asset.example:444",
		RecommendedBazelRemoteUploadLocalResults: true,
	}

	readWrite := bazelStorageSetup(response)
	if readWrite.RemoteUploadLocalResults == nil || !*readWrite.RemoteUploadLocalResults {
		t.Fatalf("read-write setup upload recommendation = %v, want true", readWrite.RemoteUploadLocalResults)
	}
	if readWrite.StorageEndpoint != response.GetStorageEndpoint() || readWrite.RemoteAssetEndpoint != response.GetRemoteAssetEndpoint() {
		t.Fatalf("read-write setup = %#v, want response endpoints", readWrite)
	}

	response.SetRecommendedBazelRemoteUploadLocalResults(false)
	readOnly := bazelStorageSetup(response)
	if readOnly.RemoteUploadLocalResults == nil || *readOnly.RemoteUploadLocalResults {
		t.Fatalf("read-only setup upload recommendation = %v, want false", readOnly.RemoteUploadLocalResults)
	}
}

func TestMakeEnsureBazelStorageClusterRequest(t *testing.T) {
	t.Parallel()

	req := makeEnsureBazelStorageClusterRequest(
		"ci",
		bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_STATIC,
		true,
		bazelv1beta.BazelStorageAccessMode_BAZEL_STORAGE_ACCESS_MODE_READ_ONLY,
	)
	if req.GetKey() != "ci" {
		t.Fatalf("key = %q, want ci", req.GetKey())
	}
	if req.GetAuthMode() != bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_STATIC {
		t.Fatalf("auth mode = %v, want static", req.GetAuthMode())
	}
	if !req.GetEnableRemoteAssetApi() {
		t.Fatal("remote asset API is disabled, want enabled")
	}
	if req.GetAccessMode() != bazelv1beta.BazelStorageAccessMode_BAZEL_STORAGE_ACCESS_MODE_READ_ONLY {
		t.Fatalf("access mode = %v, want read-only", req.GetAccessMode())
	}
}

func TestBazelStorageAccessMode(t *testing.T) {
	t.Parallel()

	if got := bazelStorageAccessMode(bazelStorageReadOnly); got != bazelv1beta.BazelStorageAccessMode_BAZEL_STORAGE_ACCESS_MODE_READ_ONLY {
		t.Fatalf("read-only access mode = %v, want read-only", got)
	}
	if got := bazelStorageAccessMode(bazelStorageReadWrite); got != bazelv1beta.BazelStorageAccessMode_BAZEL_STORAGE_ACCESS_MODE_READ_WRITE {
		t.Fatalf("read-write access mode = %v, want read-write", got)
	}
}

func TestRetryBazelProvisioning(t *testing.T) {
	t.Parallel()

	t.Run("retries transient failures", func(t *testing.T) {
		attempts := 0
		result, err := retryBazelProvisioningWithBackoff(context.Background(), &backoff.ZeroBackOff{}, func(context.Context) (string, error) {
			attempts++
			if attempts <= bazelProvisioningMaxRetries {
				return "", status.Error(codes.Unavailable, "rolling out")
			}
			return "ready", nil
		})
		if err != nil {
			t.Fatalf("retryBazelProvisioningWithBackoff: %v", err)
		}
		if result != "ready" {
			t.Fatalf("result = %q, want ready", result)
		}
		if attempts != bazelProvisioningMaxRetries+1 {
			t.Fatalf("attempts = %d, want %d", attempts, bazelProvisioningMaxRetries+1)
		}
	})

	t.Run("stops after maximum retries", func(t *testing.T) {
		attempts := 0
		_, err := retryBazelProvisioningWithBackoff(context.Background(), &backoff.ZeroBackOff{}, func(context.Context) (struct{}, error) {
			attempts++
			return struct{}{}, connect.NewError(connect.CodeUnavailable, errors.New("rolling out"))
		})
		if connect.CodeOf(err) != connect.CodeUnavailable {
			t.Fatalf("error code = %v, want unavailable", connect.CodeOf(err))
		}
		if attempts != bazelProvisioningMaxRetries+1 {
			t.Fatalf("attempts = %d, want %d", attempts, bazelProvisioningMaxRetries+1)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "gRPC resource exhausted", err: status.Error(codes.ResourceExhausted, "capacity unavailable")},
		{name: "Connect resource exhausted", err: connect.NewError(connect.CodeResourceExhausted, errors.New("capacity unavailable"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attempts := 0
			result, err := retryBazelProvisioningWithBackoff(context.Background(), &backoff.ZeroBackOff{}, func(context.Context) (string, error) {
				attempts++
				if attempts == 1 {
					return "", tc.err
				}
				return "ready", nil
			})
			if err != nil {
				t.Fatalf("retryBazelProvisioningWithBackoff: %v", err)
			}
			if result != "ready" {
				t.Fatalf("result = %q, want ready", result)
			}
			if attempts != 2 {
				t.Fatalf("attempts = %d, want 2", attempts)
			}
		})
	}

	t.Run("does not retry permanent failures", func(t *testing.T) {
		attempts := 0
		_, err := retryBazelProvisioningWithBackoff(context.Background(), &backoff.ZeroBackOff{}, func(context.Context) (struct{}, error) {
			attempts++
			return struct{}{}, status.Error(codes.InvalidArgument, "invalid")
		})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("error code = %v, want invalid argument", status.Code(err))
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	})

	t.Run("sets an overall provisioning deadline", func(t *testing.T) {
		_, err := retryBazelProvisioning(context.Background(), func(ctx context.Context) (struct{}, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("provisioning context has no deadline")
			}
			remaining := time.Until(deadline)
			if remaining < 59*time.Second || remaining > bazelProvisioningTimeout {
				t.Fatalf("provisioning deadline remaining = %v, want approximately %v", remaining, bazelProvisioningTimeout)
			}
			return struct{}{}, nil
		})
		if err != nil {
			t.Fatalf("retryBazelProvisioning: %v", err)
		}
	})

	t.Run("passes cancellation to an in-flight attempt", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		_, err := retryBazelProvisioningWithBackoff(ctx, &backoff.ZeroBackOff{}, func(callCtx context.Context) (struct{}, error) {
			attempts++
			cancel()
			<-callCtx.Done()
			return struct{}{}, callCtx.Err()
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	})
}
