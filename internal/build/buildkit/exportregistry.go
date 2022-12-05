// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/exporter/containerimage/exptypes"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func exportToRegistry(parent any, original ExportToRegistryRequest, rewritten *ExportToRegistryRequest) exporter[oci.Image] {
	if rewritten == nil {
		rewritten = &original
	}
	return &exportRegistry{parent: parent, requested: original, target: *rewritten}
}

type exportRegistry struct {
	parent    any
	requested ExportToRegistryRequest
	target    ExportToRegistryRequest

	parsed name.Repository
}

func (e *exportRegistry) Prepare(ctx context.Context) error {
	p, err := name.NewRepository(e.requested.Name, e.nameOpts()...)
	if err != nil {
		return err
	}
	e.parsed = p
	return nil
}

func (e *exportRegistry) nameOpts() []name.Option {
	if e.requested.Insecure {
		return []name.Option{name.Insecure}
	}

	return nil
}

func (e *exportRegistry) Exports() []client.ExportEntry {
	return []client.ExportEntry{{
		Type: client.ExporterImage,
		Attrs: map[string]string{
			"push":              "true",
			"name":              e.target.Name,
			"push-by-digest":    "true",
			"registry.insecure": fmt.Sprintf("%v", e.target.Insecure),
			"buildinfo":         "false", // Remove build info to keep reproducibility.
			KeySourceDateEpoch:  "0",
		},
	}}
}

func (e *exportRegistry) Provide(ctx context.Context, res *client.SolveResponse, opts clientOpts) (oci.Image, error) {
	digest, ok := res.ExporterResponse[exptypes.ExporterImageDigestKey]
	if !ok {
		return nil, fnerrors.New("digest is missing from result")
	}

	p, err := name.NewDigest(e.parsed.Name()+"@"+digest, e.nameOpts()...)
	if err != nil {
		return nil, err
	}

	img, err := compute.WithGraphLifecycle(ctx, func(ctx context.Context) (oci.Image, error) {
		return remote.Image(p, remote.WithContext(ctx))
	})
	if err != nil {
		return nil, err
	}

	if opts.SupportsCanonicalBuilds {
		return img, nil
	}

	return canonical(ctx, img)
}

func canonical(ctx context.Context, original oci.Image) (oci.Image, error) {
	img, err := tasks.Return(ctx, tasks.Action("buildkit.make-canonical"), func(ctx context.Context) (oci.Image, error) {
		return oci.WithCanonicalManifest(ctx, original)
	})
	if err != nil {
		return nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}

	cfgName, err := img.ConfigName()
	if err != nil {
		return nil, err
	}

	tasks.Attachments(ctx).AddResult("digest", digest).AddResult("config", cfgName)
	return img, nil
}
