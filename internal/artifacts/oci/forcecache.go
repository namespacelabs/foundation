// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

func LocalCopy(input NamedImage) NamedImage {
	return MakeNamedImage(input.Description(), &forceCache{desc: input.Description(), source: input.Image()})
}

type forceCache struct {
	desc   string
	source compute.Computable[Image]

	compute.LocalScoped[Image]
}

var _ compute.Computable[Image] = &forceCache{}

func (fc *forceCache) Action() *tasks.ActionEvent {
	return tasks.Action("oci.cache-image").Arg("ref", fc.desc)
}

func (fc *forceCache) Inputs() *compute.In {
	return compute.Inputs().Computable("source", fc.source)
}

func (fc *forceCache) Compute(ctx context.Context, deps compute.Resolved) (Image, error) {
	source := compute.MustGetDepValue(deps, fc.source, "source")
	cache := compute.Cache(ctx)

	var ic imageCacheable

	digest, err := ic.Cache(ctx, cache, source)
	if err != nil {
		return nil, err
	}

	result, err := ic.LoadCached(ctx, cache, nil, digest)
	if err != nil {
		return nil, err
	}

	return result.Value, nil
}
