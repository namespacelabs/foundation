// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bazelremote

import (
	"context"
	"errors"
	"testing"

	asset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	execution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func TestGRPCTarget(t *testing.T) {
	for _, test := range []struct {
		endpoint   string
		wantTarget string
		wantSecure bool
		wantError  bool
	}{
		{endpoint: "grpcs://executor.example:443", wantTarget: "executor.example:443", wantSecure: true},
		{endpoint: "grpc://cache.example:80", wantTarget: "cache.example:80", wantSecure: false},
		{endpoint: "cache.example:443", wantTarget: "cache.example:443", wantSecure: false},
		{endpoint: "grpcs://cache.example/%zz", wantError: true},
		{endpoint: "https://cache.example", wantError: true},
	} {
		t.Run(test.endpoint, func(t *testing.T) {
			gotTarget, gotSecure, err := grpcTarget(test.endpoint)
			if (err != nil) != test.wantError {
				t.Fatalf("grpcTarget(%q) error = %v, wantError = %t", test.endpoint, err, test.wantError)
			}
			if gotTarget != test.wantTarget || gotSecure != test.wantSecure {
				t.Fatalf("grpcTarget(%q) = (%q, %t), want (%q, %t)", test.endpoint, gotTarget, gotSecure, test.wantTarget, test.wantSecure)
			}
		})
	}
}

type recordingFetchClient struct {
	request  *asset.FetchBlobRequest
	response *asset.FetchBlobResponse
	err      error
}

func (c *recordingFetchClient) FetchBlob(_ context.Context, request *asset.FetchBlobRequest, _ ...grpc.CallOption) (*asset.FetchBlobResponse, error) {
	c.request = request
	return c.response, c.err
}

func (c *recordingFetchClient) FetchDirectory(context.Context, *asset.FetchDirectoryRequest, ...grpc.CallOption) (*asset.FetchDirectoryResponse, error) {
	return nil, errors.New("unexpected FetchDirectory call")
}

func TestFetchBlob(t *testing.T) {
	const (
		uri  = "https://example.com/sdk.tar.gz"
		hash = "0000000000000000000000000000000000000000000000000000000000000000"
		sri  = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
		size = 123
	)
	fetch := &recordingFetchClient{response: &asset.FetchBlobResponse{
		Status:         &status.Status{Code: int32(codes.OK)},
		BlobDigest:     &execution.Digest{Hash: hash, SizeBytes: size},
		DigestFunction: execution.DigestFunction_SHA256,
	}}

	digest, err := (&ExecutorClient{asset: fetch}).FetchBlob(context.Background(), uri, hash)
	if err != nil {
		t.Fatal(err)
	}
	if digest.Hash != hash || digest.Size != size {
		t.Errorf("digest = %s, want %s/%d", digest, hash, size)
	}
	if got := fetch.request.GetUris(); len(got) != 1 || got[0] != uri {
		t.Errorf("URIs = %q, want [%q]", got, uri)
	}
	if fetch.request.GetDigestFunction() != execution.DigestFunction_SHA256 {
		t.Errorf("digest function = %s, want SHA256", fetch.request.GetDigestFunction())
	}
	if got := fetch.request.GetQualifiers(); len(got) != 1 || got[0].GetName() != "checksum.sri" || got[0].GetValue() != sri {
		t.Errorf("qualifiers = %v, want checksum.sri=%q", got, sri)
	}
}

func TestFetchBlobPropagatesErrors(t *testing.T) {
	wantErr := errors.New("fetch failed")
	client := &ExecutorClient{asset: &recordingFetchClient{err: wantErr}}
	_, err := client.FetchBlob(context.Background(), "https://example.com/sdk.tar.gz", "0000000000000000000000000000000000000000000000000000000000000000")
	if !errors.Is(err, wantErr) {
		t.Errorf("error = %v, want %v", err, wantErr)
	}
}

func TestFetchBlobRejectsMismatchedDigest(t *testing.T) {
	fetch := &recordingFetchClient{response: &asset.FetchBlobResponse{
		Status:     &status.Status{Code: int32(codes.OK)},
		BlobDigest: &execution.Digest{Hash: "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", SizeBytes: 123},
	}}
	client := &ExecutorClient{asset: fetch}
	_, err := client.FetchBlob(context.Background(), "https://example.com/sdk.tar.gz", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected digest mismatch error")
	}
}
