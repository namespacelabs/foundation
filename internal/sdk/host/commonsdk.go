// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package host

import (
	"context"
	"fmt"
	"path/filepath"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/artifacts/unpack"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type LocalSDK struct {
	Path    string
	Version string
	Binary  string
}

type PrepareSDK struct {
	Name     string
	Platform specs.Platform
	Binary   string
	Version  string
	Ref      artifacts.Reference

	compute.DoScoped[LocalSDK]
}

func (p *PrepareSDK) Action() *tasks.ActionEvent {
	return tasks.Action(fmt.Sprintf("%s.sdk.prepare", p.Name)).Arg("version", p.Version)
}
func (p *PrepareSDK) Inputs() *compute.In {
	return compute.Inputs().
		Str("version", p.Version).
		JSON("ref", p.Ref).
		Str("name", p.Name).
		JSON("platform", p.Platform).
		Str("binary", p.Binary)
}
func (p *PrepareSDK) Output() compute.Output { return compute.Output{NotCacheable: true} }
func (p *PrepareSDK) Compute(ctx context.Context, _ compute.Resolved) (LocalSDK, error) {
	// XXX security
	// We only checksum go/bin/go, it's a robustness/performance trade-off.
	fsys := unpack.Unpack(fmt.Sprintf("%s-sdk", p.Name), tarfs.TarGunzip(download.URL(p.Ref)), unpack.WithChecksumPaths(p.Binary))

	// The contents of the sdk are unpacked here, rather than as an input to
	// this computable, as DoScoped Computables must have a deterministic set of
	// inputs; and the digest of a FS is only known after the FS is available.
	sdk, err := compute.GetValue(ctx, fsys)
	if err != nil {
		return LocalSDK{}, err
	}

	binary := filepath.Join(sdk.Files, p.Binary)
	if !IsNixOS() {
		return LocalSDK{
			Path:    sdk.Files,
			Version: p.Version,
			Binary:  binary,
		}, nil
	}

	patchedBin, err := EnsureNixosPatched(ctx, binary)
	if err != nil {
		return LocalSDK{}, err
	}

	return LocalSDK{
		Path:    sdk.Files,
		Version: p.Version,
		Binary:  patchedBin,
	}, nil
}

var _ compute.Digestible = LocalSDK{}

func (sdk LocalSDK) ComputeDigest(context.Context) (schema.Digest, error) {
	return schema.DigestOf(sdk)
}
