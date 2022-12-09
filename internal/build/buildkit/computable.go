// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/schema"
)

const (
	maxExpectedWorkspaceSize uint64 = 32 * 1024 * 1024 // 32MB should be enough for everyone (famous last words).
)

var SkipExpectedMaxWorkspaceSizeCheck = false

// XXX make this a flag instead. The assumption here is that in CI the filesystem is readonly.
var PreDigestLocalInputs = environment.IsRunningInCI()

type LocalContents struct {
	Module build.Workspace
	Path   string

	// If set, only files matching these patterns will be included in the state.
	IncludePatterns []string
	// Added to the base exclude patterns. Override include patterns: if a file matches both, it is not included.
	ExcludePatterns []string
}

func (l LocalContents) Abs() string {
	return filepath.Join(l.Module.Abs(), l.Path)
}

func precomputedReq(req *FrontendRequest, target build.BuildTarget) compute.Computable[*FrontendRequest] {
	return compute.Precomputed(req, func(ctx context.Context, req *FrontendRequest) (schema.Digest, error) {
		return digestRequest(ctx, req, target)
	})
}

func digestRequest(ctx context.Context, req *FrontendRequest, target build.BuildTarget) (schema.Digest, error) {
	var kvs []keyValue
	for k, v := range req.FrontendInputs {
		def, err := MarshalForTarget(ctx, v, target)
		if err != nil {
			return schema.Digest{}, err
		}
		kvs = append(kvs, keyValue{k, def})
	}

	// Make order stable.
	sort.Slice(kvs, func(i, j int) bool {
		return strings.Compare(kvs[i].Name, kvs[j].Name) < 0
	})

	w := sha256.New()
	for _, kv := range kvs {
		if _, err := fmt.Fprintf(w, "%s:", kv.Name); err != nil {
			return schema.Digest{}, err
		}
		if err := llb.WriteTo(kv.Value, w); err != nil {
			return schema.Digest{}, err
		}
	}

	if req.Def != nil {
		if err := llb.WriteTo(req.Def, w); err != nil {
			return schema.Digest{}, err
		}
	}

	return schema.FromHash("sha256", w), nil
}

type ClientFactory interface {
	MakeClient(context.Context) (*GatewayClient, error)
}

func MakeImage(makeClient ClientFactory, conf build.BuildTarget, req compute.Computable[*FrontendRequest], localDirs []LocalContents, targetName compute.Computable[oci.AllocatedRepository]) compute.Computable[oci.Image] {
	base := &baseRequest[oci.Image]{
		sourceLabel:    conf.SourceLabel(),
		sourcePackage:  conf.SourcePackage(),
		makeClient:     makeClient,
		targetPlatform: conf.TargetPlatform(),
		req:            req,
		localDirs:      localDirs,
	}

	return &reqToImage{baseRequest: base, targetName: targetName}
}
