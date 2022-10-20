// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/moby/buildkit/client/llb"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const (
	maxExpectedWorkspaceSize uint64 = 32 * 1024 * 1024 // 32MB should be enough for everyone.
)

var SkipExpectedMaxWorkspaceSizeCheck = false

// XXX make this a flag instead. The assumption here is that in CI the filesystem is readonly.
var PreDigestLocalInputs = environment.IsRunningInCI()

type LocalContents struct {
	Module         build.Workspace
	Path           string
	ObserveChanges bool
	// For Web we apply special handling temporarily: not including the root tsconfig.json as it belongs to Node.js
	TemporaryIsWeb bool

	// If set, only files matching these patterns will be included in the state.
	IncludePatterns []string
	// Added to the base exclude patterns. Override include patterns: if a file matches both, it is not included.
	ExcludePatterns []string
}

func (l LocalContents) Abs() string {
	return filepath.Join(l.Module.Abs(), l.Path)
}

func precomputedReq(req *frontendReq) compute.Computable[*frontendReq] {
	return compute.Precomputed(req, digestRequest)
}

func digestRequest(ctx context.Context, req *frontendReq) (schema.Digest, error) {
	var kvs []keyValue
	for k, v := range req.FrontendInputs {
		def, err := v.Marshal(ctx)
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

func makeImage(env cfg.Context, conf build.BuildTarget, req compute.Computable[*frontendReq], localDirs []LocalContents, targetName compute.Computable[oci.AllocatedName]) compute.Computable[oci.Image] {
	base := &baseRequest[oci.Image]{
		sourceLabel:    conf.SourceLabel(),
		sourcePackage:  conf.SourcePackage(),
		config:         env.Configuration(),
		targetPlatform: platformOrDefault(conf.TargetPlatform()),
		req:            req,
		localDirs:      localDirs,
	}

	return &reqToImage{baseRequest: base, targetName: targetName}
}

func platformOrDefault(targetPlatform *specs.Platform) specs.Platform {
	if targetPlatform == nil {
		return HostPlatform()
	}

	return *targetPlatform
}
